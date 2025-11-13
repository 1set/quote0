# Quote/0 Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/1set/quote0.svg)](https://pkg.go.dev/github.com/1set/quote0)
[![Go Report Card](https://goreportcard.com/badge/github.com/1set/quote0)](https://goreportcard.com/report/github.com/1set/quote0)
[![Go Version](https://img.shields.io/github/go-mod/go-version/1set/quote0)](https://github.com/1set/quote0)
[![License](https://img.shields.io/github/license/1set/quote0)](https://github.com/1set/quote0/blob/master/LICENSE)

A Go SDK and CLI for Quote/0, a Wi-Fi enabled e-ink display device with a 296×152 pixel screen. This library provides a type-safe interface to the official Text and Image APIs, handling Bearer authentication, rate limiting (1 QPS), and error normalization for both JSON and plain-text responses. Built using only the Go standard library.

**Requirements:** Go 1.18+

**API Documentation:**

- Product Info: <https://dot.mindreset.tech/product/quote>
- Text API: <https://dot.mindreset.tech/docs/service/studio/api/text_api>
- Image API: <https://dot.mindreset.tech/docs/service/studio/api/image_api>

## Features

- Bearer auth with default device ID and per-request overrides
- 1 QPS rate limiter (context-aware, pluggable)
- Robust error normalization for JSON or plain-text responses
- Idiomatic, well-documented Go API with convenience helpers
- No third-party dependencies (Go stdlib only)
- Minimal CLI for quick sends and smoke tests

## Installation

As a library:

```bash
go get github.com/1set/quote0
```

Build the CLI:

```bash
go build ./cmd/quote0
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/1set/quote0"
)

func main() {
    client, err := quote0.NewClient(
        os.Getenv("QUOTE0_TOKEN"),
        quote0.WithDefaultDeviceID(os.Getenv("QUOTE0_DEVICE")),
    )
    if err != nil { log.Fatal(err) }

    // Send text (all fields are optional except DeviceID which can be set via client default)
    _, err = client.SendText(context.Background(), quote0.TextRequest{
        RefreshNow: quote0.Bool(true),
        Title:      "Status Update",
        Message:    "Deployment finished successfully (OK)",
        Signature:  time.Now().Format("2006-01-02 15:04:05 MST"),
    })
    if err != nil { log.Fatal(err) }
}
```

## API Overview

Create a client:

- `NewClient(apiToken string, opts ...ClientOption) (*Client, error)`

Client options:

- `WithDefaultDeviceID(deviceID string)` - set default device ID
- `WithBaseURL(baseURL string)` - override host (defaults to `https://dot.mindreset.tech`)
- `WithHTTPClient(*http.Client)` - custom HTTP client
- `WithRateLimiter(RateLimiter)` - custom limiter (nil disables client-side limiting)
- `WithUserAgent(string)` - custom User-Agent

### Text API

- `SendText(ctx context.Context, req TextRequest) (*APIResponse, error)`
- `SendTextToDevice(ctx, deviceID string, req TextRequest) (*APIResponse, error)`
- `SendTextSimple(title, message string, signature ...string) (*APIResponse, error)`

TextRequest fields:

- `RefreshNow` (optional bool pointer) - immediate refresh
- `DeviceID` - filled automatically from default if omitted (required)
- `Title` - optional; displays on the first line
- `Message` - optional; displays on the next three lines
- `Signature` - optional; displays fixed at the bottom-right corner
- `Icon` - optional base64 40x40 px PNG shown at the bottom-left corner
- `Link` - optional URL opened via the Dot app

**Display Layout:**
The Quote/0 screen has a fixed layout for Text API mode. Title appears on the first line, followed by message text spanning three lines. Icon (if provided) appears at the bottom-left corner, and signature at the bottom-right. If any field is omitted, that area remains blank - the layout does not reflow or adjust responsively.

**Note:** All fields except `DeviceID` are optional. You can send a text request with only `DeviceID` to refresh the display without changing content.

### Image API

- `SendImage(ctx context.Context, req ImageRequest) (*APIResponse, error)`
- `SendImageToDevice(ctx, deviceID string, req ImageRequest) (*APIResponse, error)`
- `SendImageSimple(base64PNG string) (*APIResponse, error)`
  
In addition to sending a base64 string, the SDK can encode for you:

- Provide raw bytes: set `ImageBytes` in `ImageRequest`, or call `SendImageBytes(ctx, png, req)`.
- Provide a file path: set `ImagePath` in `ImageRequest`, or call `SendImageFile(ctx, path, req)`.

ImageRequest fields:

- `Image` - base64 296x152 px PNG (required unless `ImageBytes` or `ImagePath` is provided)
- `ImageBytes` - raw 296x152 px PNG bytes; SDK encodes to base64 (json:"-")
- `ImagePath` - path to a 296x152 px PNG file; SDK reads + encodes (json:"-")
- `Link` - optional URL
- `Border` - optional screen edge color: `BorderWhite` (default) or `BorderBlack`
- `DitherType`, `DitherKernel` - optional dithering parameters

Note: If `ditherType` is omitted, the server defaults to error diffusion with the Floyd-Steinberg kernel (equivalent to `ditherType=DIFFUSION` + `ditherKernel=FLOYD_STEINBERG`).

**Important:** `ditherKernel` is only effective when `ditherType=DIFFUSION`. When `ditherType` is `ORDERED` or `NONE`, the kernel parameter is ignored by the server.

Supported values:

- ditherType: `DIFFUSION` | `ORDERED` | `NONE`
- ditherKernel (only for DIFFUSION): `FLOYD_STEINBERG` | `ATKINSON` | `BURKES` | `SIERRA2` | `STUCKI` | `JARVIS_JUDICE_NINKE` | `DIFFUSION_ROW` | `DIFFUSION_COLUMN` | `DIFFUSION_2D` | `THRESHOLD`

Quick guidance:

- `DIFFUSION` with various kernels - error diffusion creates natural gradients. Default kernel is `FLOYD_STEINBERG` (balanced). Other kernels like `ATKINSON` (crisp), `STUCKI` (sharp), `JARVIS_JUDICE_NINKE` (smooth) offer different visual characteristics.
- `ORDERED` - fixed halftone pattern; may show grid artifacts in gradients. Kernel parameter has no effect.
- `NONE` - no dithering; recommended for text-based images. Kernel parameter has no effect.

Example with border:

```go
client.SendImage(ctx, quote0.ImageRequest{
    RefreshNow: quote0.Bool(true),
    Image:      base64EncodedPNG,
    Border:     quote0.BorderBlack,  // or quote0.BorderWhite (default)
})
```

### Error Handling

All non-2xx responses return `*quote0.APIError`:

```go
resp, err := client.SendText(ctx, req)
if err != nil {
    if apiErr, ok := err.(*quote0.APIError); ok {
        // Status code + normalized message
        log.Printf("API error: status=%d code=%s msg=%s", apiErr.StatusCode, apiErr.Code, apiErr.Message)
        if quote0.IsRateLimitError(err) { log.Print("rate limited") }
        if quote0.IsAuthError(err) { log.Print("auth error") }
    } else {
        log.Fatalf("network error: %v", err)
    }
}
```

The SDK accepts both JSON error envelopes and plain-text (including Chinese) messages.

### Rate Limit

The built-in limiter enforces 1 QPS across the client. For advanced control:

```go
client, _ := quote0.NewClient(
    token,
    quote0.WithRateLimiter(quote0.NewFixedIntervalLimiter(1500*time.Millisecond)),
)
```

## CLI Usage

Build:

```bash
go build -o quote0 ./cmd/quote0
```

Environment defaults:

- `QUOTE0_TOKEN` - API token
- `QUOTE0_DEVICE` - default device ID

Send text:

```bash
QUOTE0_TOKEN=YOUR_DOT_APP_TOKEN QUOTE0_DEVICE=YOUR_DEVICE_ID \
  ./quote0 text -title "Hello" -message "World" -signature "2025-11-08 14:00 CST"
```

Send image from file or base64:

```bash
./quote0 image -token "$QUOTE0_TOKEN" -device "$QUOTE0_DEVICE" -image-file screen.png
./quote0 image -token "$QUOTE0_TOKEN" -device "$QUOTE0_DEVICE" -image "<base64>"
```

## Notes & Limits

- Endpoints:
  - Text: `POST https://dot.mindreset.tech/api/open/text`
  - Image: `POST https://dot.mindreset.tech/api/open/image`
- API rate limit: 1 QPS. The client enforces this by default.
- This SDK is stdlib-only to keep builds simple and portable.
