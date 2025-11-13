package quote0

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the default API host. Endpoints are under /api/open/*.
	// This matches the official curl examples.
	DefaultBaseURL = "https://dot.mindreset.tech"

	textEndpoint        = "/api/open/text"
	imageEndpoint       = "/api/open/image"
	userAgentProduct    = "quote0-go-sdk"
	userAgentVersion    = "1.0"
	defaultHTTPTimeout  = 30 * time.Second
	maxResponseBodySize = 4 << 20 // 4 MiB guard
)

// APIResponse reflects a typical JSON envelope from the service.
// Some responses may be plain text; in those cases, Message/StatusCode/RawBody are set.
type APIResponse struct {
	// Code carries the numeric status returned by the Quote/0 API (0 on success).
	Code int `json:"code"`
	// Message is the string message provided by the service (e.g., "ok" or reason text).
	Message string `json:"message"`
	// Result contains the raw JSON payload (varies per endpoint; caller can unmarshal).
	Result json.RawMessage `json:"result"`
	// StatusCode keeps the HTTP status code observed for the request.
	StatusCode int `json:"-"`
	// RawBody contains the exact response bytes for troubleshooting or custom parsing.
	RawBody []byte `json:"-"`
}

// Client exposes the Quote/0 APIs with proper authentication and rate limiting.
type Client struct {
	baseURL   string
	apiKey    string
	http      *http.Client
	limiter   RateLimiter
	userAgent string

	mu            sync.RWMutex
	defaultDevice string
}

// ClientOption mutates the client during construction.
type ClientOption func(*Client)

// NewClient builds a client. apiKey is required (format: dot_app_xxx).
func NewClient(apiKey string, opts ...ClientOption) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("quote0: API token is required")
	}
	c := &Client{
		baseURL:   DefaultBaseURL,
		apiKey:    apiKey,
		userAgent: buildDefaultUserAgent(),
		http:      &http.Client{Timeout: defaultHTTPTimeout},
		limiter:   NewFixedIntervalLimiter(time.Second), // 1 QPS
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: defaultHTTPTimeout}
	}
	c.baseURL = sanitizeBaseURL(c.baseURL)
	return c, nil
}

// WithBaseURL overrides the API host (useful for staging/tests). No trailing slash required.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithHTTPClient installs a custom http.Client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.http = hc }
}

// WithRateLimiter replaces the default limiter. Pass nil to disable (not recommended).
func WithRateLimiter(l RateLimiter) ClientOption {
	return func(c *Client) { c.limiter = l }
}

// WithUserAgent sets a custom User-Agent string.
func WithUserAgent(ua string) ClientOption {
	return func(c *Client) { c.userAgent = ua }
}

// WithDefaultDeviceID sets a default device serial number used when request omits deviceId.
func WithDefaultDeviceID(deviceID string) ClientOption {
	return func(c *Client) {
		c.mu.Lock()
		c.defaultDevice = strings.TrimSpace(deviceID)
		c.mu.Unlock()
	}
}

// SetDefaultDeviceID updates the default device ID in a thread-safe manner.
func (c *Client) SetDefaultDeviceID(deviceID string) {
	c.mu.Lock()
	c.defaultDevice = strings.TrimSpace(deviceID)
	c.mu.Unlock()
}

// GetDefaultDeviceID returns the current default device ID.
func (c *Client) GetDefaultDeviceID() string {
	c.mu.RLock()
	id := c.defaultDevice
	c.mu.RUnlock()
	return id
}

func sanitizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return DefaultBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

func (c *Client) resolveDeviceID(explicit string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return explicit, nil
	}
	c.mu.RLock()
	id := c.defaultDevice
	c.mu.RUnlock()
	id = strings.TrimSpace(id)
	if id == "" {
		return "", ErrDeviceIDMissing
	}
	return id, nil
}

// doJSON encodes the payload, executes the POST, and normalizes the response.
func (c *Client) doJSON(ctx context.Context, endpoint string, payload interface{}) (*APIResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("quote0: encode request: %w", err)
	}

	url := c.baseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("quote0: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if ua := strings.TrimSpace(c.userAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("quote0: execute request: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxResponseBodySize)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("quote0: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildAPIError(resp.StatusCode, raw)
	}

	out := &APIResponse{StatusCode: resp.StatusCode, RawBody: raw}
	if len(raw) == 0 {
		return out, nil
	}

	// Try JSON first based on header; if it fails, fall back to plain text.
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "application/json") {
		if err := json.Unmarshal(raw, out); err == nil {
			return out, nil
		}
	}
	out.Message = strings.TrimSpace(string(raw))
	return out, nil
}

func buildDefaultUserAgent() string {
	goVer := strings.TrimPrefix(runtime.Version(), "go")
	if goVer == "" {
		goVer = runtime.Version()
	}
	return fmt.Sprintf("%s/%s (+https://github.com/1set/quote0; Go%s; %s/%s)",
		userAgentProduct, userAgentVersion, goVer, runtime.GOOS, runtime.GOARCH)
}
