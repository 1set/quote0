package quote0

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

var (
	// ErrDeviceIDMissing indicates deviceId is required.
	ErrDeviceIDMissing = errors.New("quote0: deviceId is required")
	// ErrImagePayloadMissing indicates image payload is required.
	ErrImagePayloadMissing = errors.New("quote0: image payload is required")
	// ErrTitleMissing indicates title is required.
	ErrTitleMissing = errors.New("quote0: title is required")
	// ErrMessageMissing indicates message is required.
	ErrMessageMissing = errors.New("quote0: message is required")
)

// APIError captures non-2xx responses. The service may return JSON or plain text (e.g. Chinese).
type APIError struct {
	StatusCode int
	// Code is a normalized string representation of server error code when present (e.g. "429").
	Code string
	// Message is a human-readable message from the server or synthesized from body.
	Message string
	// RawBody keeps the original payload for debugging.
	RawBody []byte
}

func (e *APIError) Error() string {
	b := strings.Builder{}
	b.WriteString("quote0: API error (status=")
	b.WriteString(strconv.Itoa(e.StatusCode))
	if e.Code != "" {
		b.WriteString(", code=")
		b.WriteString(e.Code)
	}
	b.WriteString(")")
	if m := strings.TrimSpace(e.Message); m != "" {
		b.WriteString(": ")
		b.WriteString(m)
	}
	return b.String()
}

// IsRateLimitError returns true if err is an APIError with HTTP status 429 (Too Many Requests).
func IsRateLimitError(err error) bool {
	if ae, ok := err.(*APIError); ok {
		return ae.StatusCode == 429
	}
	return false
}

// IsAuthError returns true if err is an APIError with HTTP status 401 or 403 (authentication/authorization failure).
func IsAuthError(err error) bool {
	if ae, ok := err.(*APIError); ok {
		return ae.StatusCode == 401 || ae.StatusCode == 403
	}
	return false
}

func buildAPIError(status int, body []byte) error {
	trimmed := strings.TrimSpace(string(body))
	ae := &APIError{StatusCode: status, RawBody: body, Message: trimmed}

	// Attempt JSON decoding if body looks like JSON
	if isJSONObject(trimmed) {
		if obj := tryParseJSON(body); obj != nil {
			extractErrorFields(ae, obj, trimmed)
		}
	}
	return ae
}

// isJSONObject checks if a string looks like a JSON object
func isJSONObject(s string) bool {
	return len(s) > 0 && strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

// tryParseJSON attempts to unmarshal body into a map, returning nil on failure
func tryParseJSON(body []byte) map[string]interface{} {
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err == nil {
		return obj
	}
	return nil
}

// extractErrorFields populates APIError fields from a parsed JSON object
func extractErrorFields(ae *APIError, obj map[string]interface{}, fallback string) {
	// Extract message from "message" or "error" field
	if v, ok := obj["message"].(string); ok && v != "" {
		ae.Message = v
	} else if v, ok := obj["error"].(string); ok && v != "" {
		ae.Message = v
	} else {
		ae.Message = fallback
	}
	// Extract code (could be string or number)
	if v, ok := obj["code"]; ok {
		ae.Code = formatCode(v)
	}
}

// formatCode converts a code field (string or number) to a string
func formatCode(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.Itoa(int(t))
	default:
		return ""
	}
}
