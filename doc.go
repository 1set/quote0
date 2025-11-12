// Package quote0 provides a Go 1.18+ SDK for Quote/0 e-ink display devices.
//
// Quote/0 is a Wi-Fi enabled e-paper display with a 296×152 pixel screen that receives content
// updates via REST API. The device maintains displayed content without power (bistable e-ink)
// and supports both text-based layouts and arbitrary image rendering. See https://dot.mindreset.tech
// for device specifications and documentation.
//
// Features
//   - Bearer token authentication
//   - Optional default device ID with per-request override
//   - 1 QPS rate limiting (pluggable, context aware)
//   - Robust error handling for JSON and plain-text (Chinese) responses
//   - No third-party dependencies (stdlib only)
//
// Official API Documentation:
//   - https://dot.mindreset.tech/docs/service/studio/api/text_api
//   - https://dot.mindreset.tech/docs/service/studio/api/image_api
package quote0
