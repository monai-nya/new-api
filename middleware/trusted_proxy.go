package middleware

import (
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const trustedProxiesEnv = "TRUSTED_PROXIES"

func ConfigureTrustedProxies(engine *gin.Engine) error {
	if engine == nil {
		return fmt.Errorf("gin engine is nil")
	}

	raw := strings.TrimSpace(os.Getenv(trustedProxiesEnv))
	if raw == "" {
		engine.ForwardedByClientIP = false
		return engine.SetTrustedProxies(nil)
	}

	parts := strings.Split(raw, ",")
	proxies := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		proxy := strings.TrimSpace(part)
		if proxy == "" {
			continue
		}
		if proxy == "0.0.0.0/0" || proxy == "::/0" {
			return fmt.Errorf("%s must not trust all addresses", trustedProxiesEnv)
		}
		if _, ok := seen[proxy]; ok {
			continue
		}
		seen[proxy] = struct{}{}
		proxies = append(proxies, proxy)
	}

	if len(proxies) == 0 {
		engine.ForwardedByClientIP = false
		return engine.SetTrustedProxies(nil)
	}
	if err := engine.SetTrustedProxies(proxies); err != nil {
		return fmt.Errorf("invalid %s: %w", trustedProxiesEnv, err)
	}
	engine.ForwardedByClientIP = true
	return nil
}
