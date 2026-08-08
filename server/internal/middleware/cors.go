package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// defaultAllowedOrigins covers local development (Vite dev server + Go server).
// Production is same-origin (embedded frontend), so no CORS is needed there.
const defaultAllowedOrigins = "http://localhost:5173,http://127.0.0.1:5173,http://localhost:8081,http://127.0.0.1:8081"

// CORS returns a Gin middleware that only reflects allowed origins.
// ALLOWED_ORIGINS env var is a comma-separated list; defaults to localhost dev.
// Mirroring an arbitrary origin with Allow-Credentials would let any website
// issue authenticated cross-origin requests from a victim's browser.
func CORS() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range strings.Split(envOrDefault("ALLOWED_ORIGINS", defaultAllowedOrigins), ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = true
		}
	}

	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" {
			if allowed[origin] {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Vary", "Origin")
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Cookie, X-Org-Slug")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
