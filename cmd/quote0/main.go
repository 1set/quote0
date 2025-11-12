// Package main provides a simple CLI for testing the Quote/0 SDK.
// It supports sending text and image content to Quote/0 e-ink devices via command-line flags.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/1set/quote0"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "text":
		err = runText(os.Args[2:])
	case "image":
		err = runImage(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
		return
	default:
		printUsage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "quote0: %v\n", err)
		os.Exit(1)
	}
}

func runText(args []string) error {
	fs := flag.NewFlagSet("text", flag.ContinueOnError)
	token := fs.String("token", os.Getenv("QUOTE0_TOKEN"), "API token; or set QUOTE0_TOKEN")
	device := fs.String("device", os.Getenv("QUOTE0_DEVICE"), "Device serial; or set QUOTE0_DEVICE")
	title := fs.String("title", "", "Title (required)")
	message := fs.String("message", "", "Message (required)")
	signature := fs.String("signature", "", "Optional signature (defaults to timestamp if empty)")
	icon := fs.String("icon", "", "Base64 40x40 PNG icon (optional)")
	iconFile := fs.String("icon-file", "", "Path to 40x40 PNG icon (optional)")
	link := fs.String("link", "", "Optional URL")
	refresh := fs.Bool("refresh", true, "Set refreshNow=true")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*token) == "" {
		return errors.New("missing API token (use -token or QUOTE0_TOKEN)")
	}
	if strings.TrimSpace(*device) == "" {
		return errors.New("missing device serial (use -device or QUOTE0_DEVICE)")
	}
	if strings.TrimSpace(*title) == "" || strings.TrimSpace(*message) == "" {
		return errors.New("-title and -message are required")
	}
	iconData, err := loadBase64(*icon, *iconFile, "icon")
	if err != nil {
		return err
	}

	client, err := quote0.NewClient(*token, quote0.WithDefaultDeviceID(*device))
	if err != nil {
		return err
	}

	sig := strings.TrimSpace(*signature)
	if sig == "" {
		sig = time.Now().Format("2006-01-02 15:04:05 MST")
	}

	req := quote0.TextRequest{
		RefreshNow: quote0.Bool(*refresh),
		Title:      *title,
		Message:    *message,
		Signature:  sig,
		Icon:       iconData,
		Link:       *link,
	}
	resp, err := client.SendText(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Text sent (code=%d message=%s)\n", resp.Code, resp.Message)
	return nil
}

func runImage(args []string) error {
	fs := flag.NewFlagSet("image", flag.ContinueOnError)
	token := fs.String("token", os.Getenv("QUOTE0_TOKEN"), "API token; or set QUOTE0_TOKEN")
	device := fs.String("device", os.Getenv("QUOTE0_DEVICE"), "Device serial; or set QUOTE0_DEVICE")
	image := fs.String("image", "", "Base64 296x152 PNG")
	imageFile := fs.String("image-file", "", "Path to 296x152 PNG (base64 encoded internally)")
	link := fs.String("link", "", "Optional URL")
	border := fs.Int("border", 0, "Border width in pixels (optional)")
	ditherType := fs.String("dither-type", "", "Dither type (NONE|DIFFUSION|ORDERED)")
	ditherKernel := fs.String("dither-kernel", "", "Dither kernel (FLOYD_STEINBERG, ATKINSON, ...)")
	refresh := fs.Bool("refresh", true, "Set refreshNow=true")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*token) == "" {
		return errors.New("missing API token (use -token or QUOTE0_TOKEN)")
	}
	if strings.TrimSpace(*device) == "" {
		return errors.New("missing device serial (use -device or QUOTE0_DEVICE)")
	}

	if strings.TrimSpace(*image) != "" && strings.TrimSpace(*imageFile) != "" {
		return fmt.Errorf("provide either -image or -image-file, not both")
	}
	if strings.TrimSpace(*image) == "" && strings.TrimSpace(*imageFile) == "" {
		return errors.New("provide -image or -image-file")
	}

	client, err := quote0.NewClient(*token, quote0.WithDefaultDeviceID(*device))
	if err != nil {
		return err
	}

	req := quote0.ImageRequest{
		RefreshNow:   quote0.Bool(*refresh),
		Link:         *link,
		Border:       quote0.BorderColor(*border),
		DitherType:   quote0.DitherType(strings.ToUpper(*ditherType)),
		DitherKernel: quote0.DitherKernel(strings.ToUpper(*ditherKernel)),
	}
	if strings.TrimSpace(*image) != "" {
		req.Image = *image
	} else {
		req.ImagePath = *imageFile
	}
	resp, err := client.SendImage(context.Background(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Image sent (code=%d message=%s)\n", resp.Code, resp.Message)
	return nil
}

func loadBase64(raw, file, label string) (string, error) {
	raw = strings.TrimSpace(raw)
	file = strings.TrimSpace(file)
	switch {
	case raw != "" && file != "":
		return "", fmt.Errorf("provide either -%s or -%s-file, not both", label, label)
	case raw != "":
		return raw, nil
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(data), nil
	default:
		return "", nil
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `quote0 - Quote/0 SDK CLI

Usage:
  quote0 text  [flags]
  quote0 image [flags]

Common flags:
  -token       API token (or set QUOTE0_TOKEN)
  -device      Device serial (or set QUOTE0_DEVICE)

Text flags:
  -title       Title (required)
  -message     Message (required)
  -signature   Signature (optional; defaults to now)
  -icon        Base64 icon (optional)
  -icon-file   Path to icon PNG (optional)
  -link        URL (optional)
  -refresh     true|false (default true)

Image flags:
  -image       Base64 296x152 PNG
  -image-file  Path to 296x152 PNG (SDK encodes base64 internally)
  -border      Screen edge color: 0=white (default), 1=black
  -dither-type NONE|DIFFUSION|ORDERED (default if omitted: DIFFUSION)
  -dither-kernel FLOYD_STEINBERG|ATKINSON|BURKES|SIERRA2|STUCKI|JARVIS_JUDICE_NINKE|DIFFUSION_ROW|DIFFUSION_COLUMN|DIFFUSION_2D|THRESHOLD

Notes:
  - ditherType and ditherKernel are case-insensitive (values are upper-cased internally).
  - If ditherType is omitted, the server uses error diffusion with the Floyd-Steinberg kernel by default.
`)
}
