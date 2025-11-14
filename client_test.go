package quote0

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSendText_SuccessJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != textEndpoint {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test" {
			t.Fatalf("auth header missing: %q", got)
		}
		if got := r.Header.Get("Accept"); got != "" {
			t.Fatalf("unexpected Accept header: %q", got)
		}
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "quote0-go-sdk/1.0") {
			t.Fatalf("unexpected UA: %q", ua)
		}
		var req TextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.DeviceID != "DEV" {
			t.Fatalf("device=%s", req.DeviceID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"message":"ok"}`)
	}))
	defer srv.Close()

	c, err := NewClient("test", WithBaseURL(srv.URL), WithRateLimiter(RateLimiterFunc(func(context.Context) error { return nil })))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SendText(context.Background(), TextRequest{DeviceID: "DEV", Title: "t", Message: "m"})
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}
}

func TestSendText_PlainErrorChinese(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, "频率过高，请稍后再试")
	}))
	defer srv.Close()

	c, err := NewClient("test", WithBaseURL(srv.URL), WithRateLimiter(RateLimiterFunc(func(context.Context) error { return nil })))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SendText(context.Background(), TextRequest{DeviceID: "DEV", Title: "t", Message: "m"})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("want APIError, got %T", err)
	}
}

func TestDefaultDeviceFallbackOverride(t *testing.T) {
	got := make([]string, 0, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req TextRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		got = append(got, req.DeviceID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer srv.Close()

	c, err := NewClient("test", WithBaseURL(srv.URL), WithRateLimiter(RateLimiterFunc(func(context.Context) error { return nil })), WithDefaultDeviceID("DEF"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SendText(context.Background(), TextRequest{Title: "t", Message: "m"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SendText(context.Background(), TextRequest{DeviceID: "OVR", Title: "t", Message: "m"}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "DEF" || got[1] != "OVR" {
		t.Fatalf("got=%v", got)
	}
}

func TestSendImage_WithBytesAndPath(t *testing.T) {
	// Prepare server that asserts image field is base64 encoded
	var gotImages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != imageEndpoint {
			t.Fatalf("path=%s", r.URL.Path)
		}
		var req ImageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		gotImages = append(gotImages, req.Image)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer srv.Close()

	c, err := NewClient("test", WithBaseURL(srv.URL), WithRateLimiter(RateLimiterFunc(func(context.Context) error { return nil })), WithDefaultDeviceID("D"))
	if err != nil {
		t.Fatal(err)
	}

	// 1) Raw bytes
	png := []byte{0x89, 0x50, 0x4E, 0x47}
	if _, err := c.SendImage(context.Background(), ImageRequest{ImageBytes: png}); err != nil {
		t.Fatal(err)
	}

	// 2) File path
	tmp, err := os.CreateTemp(t.TempDir(), "img-*.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.Write(png); err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	if _, err := c.SendImage(context.Background(), ImageRequest{ImagePath: tmp.Name()}); err != nil {
		t.Fatal(err)
	}

	if len(gotImages) != 2 {
		t.Fatalf("got %d requests", len(gotImages))
	}
	// Expect base64 of the provided bytes
	expected := base64.StdEncoding.EncodeToString(png)
	if gotImages[0] != expected || gotImages[1] != expected {
		t.Fatalf("unexpected encoded images: %v", gotImages)
	}
}

func TestSendText_AllFieldsOptional(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer srv.Close()

	c, err := NewClient("test", WithBaseURL(srv.URL), WithDefaultDeviceID("DEF"), WithRateLimiter(nil))
	if err != nil {
		t.Fatal(err)
	}
	// All fields except DeviceID are optional
	if _, err := c.SendText(context.Background(), TextRequest{Message: "body"}); err != nil {
		t.Fatalf("SendText with only message should succeed: %v", err)
	}
	if _, err := c.SendText(context.Background(), TextRequest{Title: "t"}); err != nil {
		t.Fatalf("SendText with only title should succeed: %v", err)
	}
	// Empty text request (just refresh)
	if _, err := c.SendText(context.Background(), TextRequest{}); err != nil {
		t.Fatalf("SendText with no fields should succeed: %v", err)
	}
}

func TestSendImage_MissingPayload(t *testing.T) {
	c, err := NewClient("test", WithDefaultDeviceID("DEF"), WithRateLimiter(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SendImage(context.Background(), ImageRequest{}); err != ErrImagePayloadMissing {
		t.Fatalf("expected ErrImagePayloadMissing, got %v", err)
	}
}

func TestSendTextSimple_VariadicSignature(t *testing.T) {
	sigs := make([]string, 0, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req TextRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		sigs = append(sigs, req.Signature)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer srv.Close()

	c, err := NewClient("token",
		WithBaseURL(srv.URL),
		WithDefaultDeviceID("DEF"),
		WithRateLimiter(RateLimiterFunc(func(context.Context) error { return nil })))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SendTextSimple("Title", "Message", "Signed"); err != nil {
		t.Fatalf("SendTextSimple with signature: %v", err)
	}
	if _, err := c.SendTextSimple("Title", "Message"); err != nil {
		t.Fatalf("SendTextSimple without signature: %v", err)
	}
	if len(sigs) != 2 || sigs[0] != "Signed" || sigs[1] != "" {
		t.Fatalf("unexpected signatures: %v", sigs)
	}
}

func TestNewClientRequiresToken(t *testing.T) {
	if _, err := NewClient(" "); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestBuildAPIErrorJSON(t *testing.T) {
	err := buildAPIError(400, []byte(`{"code":"E100","message":"boom"}`))
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "E100" || apiErr.Message != "boom" || apiErr.StatusCode != 400 {
		t.Fatalf("unexpected api error: %+v", apiErr)
	}
}

func TestBorderColor_JSONSerialization(t *testing.T) {
	// Test that BorderColor serializes to int in JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ImageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Verify border is sent as integer
		if req.Border != BorderBlack {
			t.Fatalf("expected BorderBlack (1), got %d", req.Border)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer srv.Close()

	c, err := NewClient("test", WithBaseURL(srv.URL), WithRateLimiter(nil), WithDefaultDeviceID("D"))
	if err != nil {
		t.Fatal(err)
	}

	// Send with BorderBlack
	_, err = c.SendImage(context.Background(), ImageRequest{
		Image:  "aGVsbG8=",
		Border: BorderBlack,
	})
	if err != nil {
		t.Fatalf("SendImage: %v", err)
	}
}

func TestBorderColor_Constants(t *testing.T) {
	// Verify constant values match API spec
	if BorderWhite != 0 {
		t.Errorf("BorderWhite should be 0, got %d", BorderWhite)
	}
	if BorderBlack != 1 {
		t.Errorf("BorderBlack should be 1, got %d", BorderBlack)
	}
}

func TestTextRequest_OmitEmptyFields(t *testing.T) {
	// Verify that empty title and signature are omitted from JSON
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer srv.Close()

	c, err := NewClient("test", WithBaseURL(srv.URL), WithDefaultDeviceID("DEF"), WithRateLimiter(nil))
	if err != nil {
		t.Fatal(err)
	}

	// Send request with no title or signature
	_, err = c.SendText(context.Background(), TextRequest{
		Message: "hello",
	})
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}

	// Parse the captured body
	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	// Verify that title and signature keys are NOT present
	if _, exists := body["title"]; exists {
		t.Error("empty title should be omitted from JSON")
	}
	if _, exists := body["signature"]; exists {
		t.Error("empty signature should be omitted from JSON")
	}
	// Verify that message IS present
	if _, exists := body["message"]; !exists {
		t.Error("message should be present in JSON")
	}
	if msg, ok := body["message"].(string); !ok || msg != "hello" {
		t.Errorf("expected message='hello', got %v", body["message"])
	}
}

func TestRateLimiter_InvalidInterval(t *testing.T) {
	// Test that NewFixedIntervalLimiter handles invalid (0 or negative) intervals
	limiter := NewFixedIntervalLimiter(0)
	if limiter == nil {
		t.Fatal("NewFixedIntervalLimiter(0) should not return nil")
	}

	// Verify it still works (should default to 1 second)
	ctx := context.Background()
	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("Wait should succeed even with 0 interval: %v", err)
	}

	// Test negative interval
	limiter2 := NewFixedIntervalLimiter(-5 * time.Second)
	if limiter2 == nil {
		t.Fatal("NewFixedIntervalLimiter(-5s) should not return nil")
	}
	if err := limiter2.Wait(ctx); err != nil {
		t.Fatalf("Wait should succeed with negative interval: %v", err)
	}
}

// TestDefaultUserAgent verifies that when WithUserAgent is NOT used,
// the client uses the SDK's default User-Agent string.
func TestDefaultUserAgent(t *testing.T) {
	var receivedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"message":"ok"}`)
	}))
	defer srv.Close()

	// Create client WITHOUT setting WithUserAgent - should use default
	c, err := NewClient("test-token",
		WithBaseURL(srv.URL),
		WithRateLimiter(nil),
		WithDefaultDeviceID("TEST"),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	// Send a request
	_, err = c.SendText(context.Background(), TextRequest{
		Message: "Test default UA",
	})
	if err != nil {
		t.Fatalf("SendText failed: %v", err)
	}

	// Log the actual User-Agent received
	t.Logf("Received User-Agent: %s", receivedUA)

	// Verify default User-Agent was used
	if !strings.Contains(receivedUA, "quote0-go-sdk/1.0") {
		t.Errorf("Expected default SDK user agent containing 'quote0-go-sdk/1.0', got: %s", receivedUA)
	}
	if !strings.Contains(receivedUA, "Go") {
		t.Errorf("Expected user agent to contain Go version, got: %s", receivedUA)
	}
	// Verify it's not the Go http client default
	if receivedUA == "Go-http-client/1.1" {
		t.Error("Should use SDK default UA, not Go http client default")
	}
}

// TestAllWithOptionsNil tests that the client handles all With options being set to nil/empty gracefully.
// This ensures defensive coding and proper fallback to defaults.
func TestAllWithOptionsNil(t *testing.T) {
	// Create server to validate requests still work
	requestCount := 0
	var receivedUAs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUAs = append(receivedUAs, r.Header.Get("User-Agent"))
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"message":"ok"}`)
	}))
	defer srv.Close()

	// Test 1: Client with all With options set to nil or empty values
	c1, err := NewClient("test-token",
		WithHTTPClient(nil),     // nil HTTP client - should use default
		WithRateLimiter(nil),    // nil rate limiter - should disable rate limiting
		WithUserAgent(""),       // empty user agent - should NOT send custom UA
		WithDefaultDeviceID(""), // empty device ID - should remain empty
		WithBaseURL(srv.URL),    // test server
	)
	if err != nil {
		t.Fatalf("NewClient with nil options failed: %v", err)
	}

	// Verify that internal defaults were set correctly
	if c1.http == nil {
		t.Fatal("HTTP client should have default, got nil")
	}
	if c1.limiter != nil {
		t.Fatal("Rate limiter should be nil when passed nil, got non-nil")
	}
	// When WithUserAgent("") is explicitly called, it overrides the default to empty
	if c1.userAgent != "" {
		t.Fatalf("User-Agent should be empty when explicitly set to empty, got: %s", c1.userAgent)
	}

	// Test that sending works with explicit device ID
	_, err = c1.SendText(context.Background(), TextRequest{
		DeviceID: "EXPLICIT",
		Message:  "Empty UA test",
	})
	if err != nil {
		t.Fatalf("SendText should work with empty UA: %v", err)
	}

	// Log the actual User-Agent received
	t.Logf("Test 1 - Empty UA: Received User-Agent: '%s'", receivedUAs[0])

	// When UA is explicitly set to empty string, it sends an empty User-Agent
	// (not Go's default "Go-http-client/1.1")
	if len(receivedUAs) != 1 || receivedUAs[0] != "" {
		t.Errorf("Expected empty user agent when explicitly set to empty, got: '%s'", receivedUAs[0])
	}

	// Test 2: Client WITHOUT WithUserAgent (should use SDK default)
	receivedUAs = receivedUAs[:0] // clear
	c2, err := NewClient("test-token",
		WithBaseURL(srv.URL),
		WithRateLimiter(nil),
		WithDefaultDeviceID("DEV2"),
		// Note: NOT calling WithUserAgent - should use SDK default
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c2.SendText(context.Background(), TextRequest{
		Message: "Default UA test",
	})
	if err != nil {
		t.Fatalf("SendText failed: %v", err)
	}

	// Log the actual User-Agent received
	t.Logf("Test 2 - Default UA: Received User-Agent: %s", receivedUAs[0])

	// Should use SDK default UA, not Go's default
	if len(receivedUAs) != 1 {
		t.Fatalf("Expected 1 request, got %d", len(receivedUAs))
	}
	if !strings.Contains(receivedUAs[0], "quote0-go-sdk") {
		t.Errorf("Expected SDK default UA when not set, got: %s", receivedUAs[0])
	}

	// Verify that without default device ID, sending fails appropriately
	_, err = c1.SendText(context.Background(), TextRequest{
		Title:   "Test",
		Message: "No device test",
	})
	if err != ErrDeviceIDMissing {
		t.Errorf("Expected ErrDeviceIDMissing, got: %v", err)
	}

	// Test nil context handling (uses c2 which has default device)
	_, err = c2.SendText(nil, TextRequest{
		Message: "Nil context test",
	})
	if err != nil {
		t.Fatalf("SendText should handle nil context: %v", err)
	}
}
