package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

// AdminAuth validates that the authenticated user is an admin.
// Must be used AFTER Auth middleware (which sets "user_id").
func AdminAuth(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		var exists bool
		err := db.NewRaw("SELECT EXISTS(SELECT 1 FROM admins WHERE user_id = ?)", userID).
			Scan(c.Request.Context(), &exists)
		if err != nil || !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}

		c.Set("is_admin", true)
		c.Next()
	}
}
