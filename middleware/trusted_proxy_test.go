package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clientIPResponse(t *testing.T, engine *gin.Engine, remoteAddr string, forwardedFor string) string {
	t.Helper()
	engine.GET("/ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ip", nil)
	request.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		request.Header.Set("X-Forwarded-For", forwardedFor)
	}
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	return recorder.Body.String()
}

func TestConfigureTrustedProxiesIgnoresForwardedHeadersByDefault(t *testing.T) {
	t.Setenv(trustedProxiesEnv, "")
	engine := gin.New()
	require.NoError(t, ConfigureTrustedProxies(engine))

	clientIP := clientIPResponse(t, engine, "198.51.100.20:12345", "203.0.113.9")

	assert.Equal(t, "198.51.100.20", clientIP)
}

func TestConfigureTrustedProxiesUsesHeadersOnlyFromAllowlistedProxy(t *testing.T) {
	t.Setenv(trustedProxiesEnv, "127.0.0.1")
	engine := gin.New()
	require.NoError(t, ConfigureTrustedProxies(engine))

	clientIP := clientIPResponse(t, engine, "127.0.0.1:12345", "203.0.113.9")

	assert.Equal(t, "203.0.113.9", clientIP)
}

func TestConfigureTrustedProxiesIgnoresHeadersFromUntrustedRemote(t *testing.T) {
	t.Setenv(trustedProxiesEnv, "127.0.0.1")
	engine := gin.New()
	require.NoError(t, ConfigureTrustedProxies(engine))

	clientIP := clientIPResponse(t, engine, "198.51.100.20:12345", "203.0.113.9")

	assert.Equal(t, "198.51.100.20", clientIP)
}

func TestConfigureTrustedProxiesRejectsTrustAllNetworks(t *testing.T) {
	for _, value := range []string{"0.0.0.0/0", "::/0"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(trustedProxiesEnv, value)
			engine := gin.New()

			err := ConfigureTrustedProxies(engine)

			require.Error(t, err)
		})
	}
}
