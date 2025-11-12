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
		io.WriteString(w, `{"code":0,"message":"ok"}`)
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
		io.WriteString(w, "频率过高，请稍后再试")
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
		json.NewDecoder(r.Body).Decode(&req)
		got = append(got, req.DeviceID)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0}`)
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
		io.WriteString(w, `{"code":0}`)
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

func TestSendText_MissingTitleMessage(t *testing.T) {
	c, err := NewClient("test", WithDefaultDeviceID("DEF"), WithRateLimiter(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SendText(context.Background(), TextRequest{Message: "body"}); err != ErrTitleMissing {
		t.Fatalf("expected ErrTitleMissing, got %v", err)
	}
	if _, err := c.SendText(context.Background(), TextRequest{Title: "t"}); err != ErrMessageMissing {
		t.Fatalf("expected ErrMessageMissing, got %v", err)
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
		json.NewDecoder(r.Body).Decode(&req)
		sigs = append(sigs, req.Signature)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0}`)
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
		io.WriteString(w, `{"code":0}`)
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
