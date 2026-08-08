package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

type bearerSession struct {
	bun.BaseModel `bun:"table:sessions"`
	Token         string `bun:"token"`
}

// SignOut returns a handler that deletes the session identified by the Bearer token
// or cookie, then clears the auth cookie.
// This is a custom alternative to Authula's cookie-based sign-out.
func SignOut(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		var fromCookie bool

		if token == "" {
			cookie, err := c.Cookie("authula.session_token")
			if err == nil && cookie != "" {
				token = cookie
				fromCookie = true
			}
		}

		if token == "" {
			Err(c, http.StatusUnauthorized, "missing authorization")
			return
		}

		// Cookie contains raw token, but DB stores SHA-256 hash
		if fromCookie {
			sum := sha256.Sum256([]byte(token))
			token = hex.EncodeToString(sum[:])
		}

		_, err := db.NewDelete().
			Model(&bearerSession{}).
			Where("token = ?", token).
			Exec(c.Request.Context())
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to sign out")
			return
		}

		// Clear the auth cookie
		c.SetCookie("authula.session_token", "", -1, "/", "", false, true)

		Success(c, gin.H{"message": "signed out"})
	}
}

func extractBearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
