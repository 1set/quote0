package quote0

import (
	"context"
	"strings"
)

// TextRequest matches the /api/open/text payload.
// Only DeviceID is required; all other fields are optional.
//
// Display Layout:
// The Quote/0 screen has a fixed layout (296x152 pixels):
//   - Title: displays on the first line
//   - Message: displays on the next three lines
//   - Icon: 40x40px at the bottom-left corner
//   - Signature: fixed at the bottom-right corner
//
// If any field is omitted, that area remains blank. The layout does not reflow or adjust responsively.
type TextRequest struct {
	// RefreshNow toggles an immediate refresh on the targeted Quote/0 display. Optional.
	RefreshNow *bool `json:"refreshNow,omitempty"`
	// DeviceID is the Quote/0 serial number (hexadecimal string). Required. Leave empty to use the client's default.
	DeviceID string `json:"deviceId"`
	// Title displays on the first line. Optional.
	Title string `json:"title,omitempty"`
	// Message displays on the next three lines. Optional.
	Message string `json:"message,omitempty"`
	// Signature displays fixed at the bottom-right corner. Optional.
	Signature string `json:"signature,omitempty"`
	// Icon is a base64-encoded 40x40px PNG shown at the bottom-left corner. Optional.
	Icon string `json:"icon,omitempty"`
	// Link is an optional URL that the Quote/0 companion app can open when interacting with the device.
	Link string `json:"link,omitempty"`
}

func (r TextRequest) validate() error {
	if strings.TrimSpace(r.DeviceID) == "" {
		return ErrDeviceIDMissing
	}
	return nil
}

// SendText sends text content. If DeviceID is empty, the client's default device is used.
func (c *Client) SendText(ctx context.Context, payload TextRequest) (*APIResponse, error) {
	did, err := c.resolveDeviceID(payload.DeviceID)
	if err != nil {
		return nil, err
	}
	payload.DeviceID = did
	if err := payload.validate(); err != nil {
		return nil, err
	}
	return c.doJSON(ctx, textEndpoint, payload)
}

// SendTextToDevice is a convenience to target a specific device.
func (c *Client) SendTextToDevice(ctx context.Context, deviceID string, payload TextRequest) (*APIResponse, error) {
	payload.DeviceID = deviceID
	return c.SendText(ctx, payload)
}

// SendTextSimple is a convenience helper using Background context and immediate refresh.
// Title and message are optional. Signature is variadic; when omitted, no signature is sent.
func (c *Client) SendTextSimple(title, message string, signature ...string) (*APIResponse, error) {
	sig := ""
	if len(signature) > 0 {
		sig = signature[0]
	}
	return c.SendText(context.Background(), TextRequest{
		RefreshNow: Bool(true),
		Title:      title,
		Message:    message,
		Signature:  sig,
	})
}
