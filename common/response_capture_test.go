package common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCaptureResponseBodyEnforcesConfiguredLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldEnabled := LogResponseBodyEnabled
	oldMaxKB := LogResponseBodyMaxKB
	t.Cleanup(func() {
		LogResponseBodyEnabled = oldEnabled
		LogResponseBodyMaxKB = oldMaxKB
	})
	LogResponseBodyEnabled = true
	LogResponseBodyMaxKB = 1

	tests := []struct {
		name             string
		responseSize     int
		expectedCaptured int
		expectedExceeded bool
	}{
		{name: "at limit", responseSize: 1024, expectedCaptured: 1024, expectedExceeded: false},
		{name: "over limit", responseSize: 1025, expectedCaptured: 1024, expectedExceeded: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var captured []byte
			var exceeded bool
			router := gin.New()
			router.Use(CaptureResponseBody())
			router.GET("/", func(c *gin.Context) {
				c.String(http.StatusOK, strings.Repeat("x", test.responseSize))
				captured, exceeded = GetCapturedResponseBody(c)
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			router.ServeHTTP(recorder, request)

			assert.Len(t, recorder.Body.Bytes(), test.responseSize)
			assert.Len(t, captured, test.expectedCaptured)
			assert.Equal(t, test.expectedExceeded, exceeded)
		})
	}
}
