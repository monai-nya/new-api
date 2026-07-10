package common

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

// KeyCapturedResponseBody is the gin-context key holding the bounded response
// capture used by usage logs.
const KeyCapturedResponseBody = "key_captured_response_body"

type responseCapture struct {
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func (c *responseCapture) Write(p []byte) {
	if c.exceeded {
		return
	}
	remaining := c.limit - c.buf.Len()
	if len(p) <= remaining {
		_, _ = c.buf.Write(p)
		return
	}
	if remaining > 0 {
		_, _ = c.buf.Write(p[:remaining])
	}
	c.exceeded = true
}

type responseTee struct {
	gin.ResponseWriter
	capture *responseCapture
}

func (w *responseTee) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.capture.Write(b[:n])
	return n, err
}

func (w *responseTee) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.capture.Write([]byte(s[:n]))
	return n, err
}

// GetCapturedResponseBody returns the captured bytes and whether the configured
// limit was exceeded. The bytes.Buffer case keeps compatibility with contexts
// created by older middleware or tests.
func GetCapturedResponseBody(c *gin.Context) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(KeyCapturedResponseBody)
	if !ok || value == nil {
		return nil, false
	}
	switch capture := value.(type) {
	case *responseCapture:
		return capture.buf.Bytes(), capture.exceeded
	case *bytes.Buffer:
		return capture.Bytes(), false
	default:
		return nil, false
	}
}

func CaptureResponseBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !LogResponseBodyEnabled || !IsValidLogBodySizeKB(LogResponseBodyMaxKB) {
			c.Next()
			return
		}
		capture := &responseCapture{limit: LogResponseBodyMaxKB << 10}
		c.Set(KeyCapturedResponseBody, capture)
		c.Writer = &responseTee{ResponseWriter: c.Writer, capture: capture}
		c.Next()
	}
}
