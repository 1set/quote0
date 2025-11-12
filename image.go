package quote0

import (
	"context"
	"strings"
)

// BorderColor controls the screen edge color on the Quote/0 display.
type BorderColor int

const (
	// BorderWhite renders a white border around the display (default).
	BorderWhite BorderColor = 0
	// BorderBlack renders a black border around the display.
	BorderBlack BorderColor = 1
)

// DitherType enumerates server-accepted dithering modes.
//
// Server behavior (per official docs):
// - If ditherType is omitted, the default is error diffusion using the Floyd-Steinberg kernel.
// - You can disable dithering by selecting NONE.
// - DIFFUSION supports a set of diffusion kernels (see DitherKernel).
// - ORDERED applies an ordered/threshold-matrix halftoning pattern.
type DitherType string

const (
	// DitherNone disables dithering entirely; pixels are binarized by a simple threshold.
	// Suitable for high-contrast line art or when you require crisp edges without added grain,
	// but gradients will posterize.
	DitherNone DitherType = "NONE"
	// DitherDiffusion enables error-diffusion dithering. Each pixel's quantization error
	// is distributed to neighbors based on a selected kernel, producing smooth tonal
	// transitions with natural-looking noise. Good general-purpose choice for photos/text.
	DitherDiffusion DitherType = "DIFFUSION"
	// DitherOrdered enables ordered dithering via a threshold matrix (Bayer-like). This
	// yields a regular halftone pattern that preserves uniform regions and text edges well,
	// but can reveal grid artifacts in smooth gradients.
	DitherOrdered DitherType = "ORDERED"
)

// DitherKernel enumerates supported dithering kernels/algorithms.
//
// When DitherType is DIFFUSION, these control how quantization error is spread:
// - KernelFloydSteinberg: Classic 3x3 diffusion (7/16, 3/16, 5/16, 1/16). Balanced detail and smoothness.
// - KernelAtkinson: Lighter diffusion footprint; preserves micro-detail and text; slightly lighter images.
// - KernelBurkes: Row-oriented diffusion similar to Stucki but lighter weights; sharp edges, fine detail.
// - KernelSierra2: Sierra-2 variant; smooth gradients with moderate grain; balanced look.
// - KernelStucki: Larger footprint; strong contrast and crisp edges; can introduce more visible grain.
// - KernelJarvisJudiceNinke: Large footprint; very smooth gradients, may blur fine details slightly.
// - KernelDiffusionRow: Directional diffusion along rows; preserves horizontal detail; may create row texture.
// - KernelDiffusionColumn: Directional diffusion along columns; preserves vertical detail; may create column texture.
// - KernelDiffusion2D: More isotropic spread across 2D neighborhood; balances artifacts across directions.
//
// When DitherType is ORDERED, the following kernel is meaningful:
// - KernelThreshold: Simple threshold matrix (ordered) without diffusion; produces a stable halftone pattern.
type DitherKernel string

const (
	// KernelThreshold applies ordered dithering threshold matrix; no error diffusion; strong halftone look.
	KernelThreshold DitherKernel = "THRESHOLD"
	// KernelAtkinson uses compact diffusion pattern; good for text and small features; lighter tones.
	KernelAtkinson DitherKernel = "ATKINSON"
	// KernelBurkes applies diffusion with emphasis on immediate neighbors along the row; sharp, detailed output.
	KernelBurkes DitherKernel = "BURKES"
	// KernelFloydSteinberg is the default, classic diffusion; natural gradients and balanced detail/grain.
	KernelFloydSteinberg DitherKernel = "FLOYD_STEINBERG"
	// KernelSierra2 applies Sierra-2 diffusion; smooth gradients; moderate granularity.
	KernelSierra2 DitherKernel = "SIERRA2"
	// KernelStucki uses larger kernel; increases perceived sharpness; can be grainier.
	KernelStucki DitherKernel = "STUCKI"
	// KernelJarvisJudiceNinke produces very smooth gradients due to large kernel; may soften fine details.
	KernelJarvisJudiceNinke DitherKernel = "JARVIS_JUDICE_NINKE"
	// KernelDiffusionRow applies directional diffusion horizontally; preserves horizontal features.
	KernelDiffusionRow DitherKernel = "DIFFUSION_ROW"
	// KernelDiffusionColumn applies directional diffusion vertically; preserves vertical features.
	KernelDiffusionColumn DitherKernel = "DIFFUSION_COLUMN"
	// KernelDiffusion2D spreads error across a 2D neighborhood; balances directional artifacts.
	KernelDiffusion2D DitherKernel = "DIFFUSION_2D"
)

// ImageRequest matches the /api/open/image payload.
type ImageRequest struct {
	// RefreshNow toggles an immediate display refresh for the targeted screen.
	RefreshNow *bool `json:"refreshNow,omitempty"`
	// DeviceID is the Quote/0 serial number (hexadecimal string). Leave empty to use the client's default device.
	DeviceID string `json:"deviceId"`
	// Image is a base64-encoded 296x152px PNG payload as required by the server.
	// You normally do not need to set this directly when using ImageBytes or ImagePath.
	Image string `json:"image"`
	// ImageBytes allows providing raw 296x152px PNG bytes; the SDK will base64-encode internally.
	ImageBytes []byte `json:"-"`
	// ImagePath allows providing a file path to a 296x152px PNG; the SDK will read and base64-encode internally.
	ImagePath string `json:"-"`
	// Link is an optional URL opened inside the Quote/0 companion app.
	Link string `json:"link,omitempty"`
	// Border selects the screen edge color. Use BorderWhite (default) or BorderBlack.
	Border BorderColor `json:"border,omitempty"`
	// DitherType selects the server-side dithering strategy for tone reproduction.
	DitherType DitherType `json:"ditherType,omitempty"`
	// DitherKernel narrows the algorithm used when DitherType=DIFFUSION or ORDERED.
	DitherKernel DitherKernel `json:"ditherKernel,omitempty"`
}

func (r ImageRequest) validate() error {
	if strings.TrimSpace(r.DeviceID) == "" {
		return ErrDeviceIDMissing
	}
	if strings.TrimSpace(r.Image) == "" {
		return ErrImagePayloadMissing
	}
	return nil
}

// SendImage uploads a base64-encoded image to the device. If DeviceID is empty, the
// client's default device is used.
func (c *Client) SendImage(ctx context.Context, payload ImageRequest) (*APIResponse, error) {
	did, err := c.resolveDeviceID(payload.DeviceID)
	if err != nil {
		return nil, err
	}
	payload.DeviceID = did
	// Normalize image data: precedence -> Image (base64) > ImageBytes > ImagePath
	if strings.TrimSpace(payload.Image) == "" {
		if len(payload.ImageBytes) > 0 {
			payload.Image = encodeBase64(payload.ImageBytes)
		} else if p := strings.TrimSpace(payload.ImagePath); p != "" {
			data, readErr := readFile(p)
			if readErr != nil {
				return nil, readErr
			}
			payload.Image = encodeBase64(data)
		}
	}
	if err := payload.validate(); err != nil {
		return nil, err
	}
	return c.doJSON(ctx, imageEndpoint, payload)
}

// SendImageToDevice is a convenience to target a specific device.
func (c *Client) SendImageToDevice(ctx context.Context, deviceID string, payload ImageRequest) (*APIResponse, error) {
	payload.DeviceID = deviceID
	return c.SendImage(ctx, payload)
}

// SendImageSimple sends an image with default device and immediate refresh using Background context.
func (c *Client) SendImageSimple(base64PNG string) (*APIResponse, error) {
	return c.SendImage(context.Background(), ImageRequest{
		RefreshNow: Bool(true),
		Image:      base64PNG,
	})
}

// SendImageBytes is a convenience that accepts raw PNG bytes and performs base64 encoding internally.
func (c *Client) SendImageBytes(ctx context.Context, png []byte, meta ImageRequest) (*APIResponse, error) {
	meta.Image = ""
	meta.ImageBytes = png
	return c.SendImage(ctx, meta)
}

// SendImageFile is a convenience that accepts a PNG file path and performs base64 encoding internally.
func (c *Client) SendImageFile(ctx context.Context, path string, meta ImageRequest) (*APIResponse, error) {
	meta.Image = ""
	meta.ImagePath = path
	return c.SendImage(ctx, meta)
}
