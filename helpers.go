package quote0

import (
	"encoding/base64"
	"fmt"
	"os"
)

// Bool returns a pointer to a bool.
func Bool(v bool) *bool { return &v }

// Int returns a pointer to an int.
func Int(v int) *int { return &v }

func encodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("quote0: read image file: %w", err)
	}
	return data, nil
}
