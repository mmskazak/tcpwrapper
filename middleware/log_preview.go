package middleware

import (
	"context"
	"fmt"

	"github.com/mmskazak/tcpwrapper"
)

// LogMiddleware creates a middleware that logs the message length and first N bytes
func LogMiddleware(prefix string, maxPreviewBytes int) tcpwrapper.Middleware {
	return func(ctx context.Context, data []byte) ([]byte, error) {
		// Check if context is already cancelled
		select {
		case <-ctx.Done():
			return data, ctx.Err()
		default:
		}

		previewLen := len(data)
		if previewLen > maxPreviewBytes {
			previewLen = maxPreviewBytes
		}
		fmt.Printf("[%s] Message length: %d bytes, Preview: %s\n",
			prefix,
			len(data),
			string(data[:previewLen]))
		return data, nil
	}
}
