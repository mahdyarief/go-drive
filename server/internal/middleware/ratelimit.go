package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateWindow struct {
	start time.Time
	count int
}

// rateLimiter is a fixed-window counter per client IP. In-memory only;
// fine for a single-binary deploy, not for horizontal scaling.
type rateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string]*rateWindow
}

// RateLimit returns a middleware that allows at most limit requests per
// window per client IP, returning 429 when exceeded.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	rl := &rateLimiter{
		limit:    limit,
		window:   window,
		requests: make(map[string]*rateWindow),
	}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		rl.mu.Lock()
		entry, ok := rl.requests[ip]
		if !ok || now.Sub(entry.start) >= rl.window {
			entry = &rateWindow{start: now}
			rl.requests[ip] = entry
		}
		entry.count++
		over := entry.count > rl.limit
		rl.mu.Unlock()

		if over {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
