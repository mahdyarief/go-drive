package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets hardening headers on every response.
// CSP is opt-in via the CSP_ENABLED env var because Vite's dev HMR injects
// inline scripts; in production (embedded frontend) it should be enabled.
func SecurityHeaders() gin.HandlerFunc {
	cspEnabled := os.Getenv("CSP_ENABLED") == "1"

	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if cspEnabled {
			c.Header("Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self' ws:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
