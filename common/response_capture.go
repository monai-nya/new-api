package common

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

// KeyCapturedResponseBody is the gin-context key holding the *bytes.Buffer that
// accumulates the response body copied by responseTee. model/log.go reads it to
// populate other.admin_info.response_body.
const KeyCapturedResponseBody = "key_captured_response_body"

// responseTee wraps gin.ResponseWriter and mirrors every byte written to the
// client into buf, so the full response body (streamed SSE or non-streamed JSON)
// is available for logging. Only Write/WriteString are overridden; Flush/Hijack/
// Header etc. are promoted from the embedded gin.ResponseWriter.
type responseTee struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *responseTee) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseTee) WriteString(s string) (int, error) {
	w.buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// CaptureResponseBody is middleware that, when LogResponseBodyEnabled is on,
// replaces c.Writer with a responseTee and stores the capture buffer in the gin
// context under KeyCapturedResponseBody. Usage log recording then reads it.
func CaptureResponseBody() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !LogResponseBodyEnabled {
			c.Next()
			return
		}
		buf := &bytes.Buffer{}
		c.Set(KeyCapturedResponseBody, buf)
		c.Writer = &responseTee{ResponseWriter: c.Writer, buf: buf}
		c.Next()
	}
}
