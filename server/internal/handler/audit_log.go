package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"

	"go-drive/server/internal/store"
)

// auditLogLimit caps how many audit entries are returned per request.
const auditLogLimit = 100

// ListAuditLogs returns the current user's recent audit entries (newest first).
func ListAuditLogs(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		logs, err := store.ListAuditLogs(ctx, tx, userID, auditLogLimit)
		if err != nil {
			Err(c, http.StatusInternalServerError, "listing audit logs: "+err.Error())
			return
		}
		Success(c, gin.H{"logs": logs})
	}
}

// auditLog records a best-effort audit entry; failures are logged and never
// fail the main operation.
func auditLog(ctx context.Context, tx bun.IDB, userID, action, entityType, entityID string, metadata map[string]any) {
	if err := store.CreateAuditLog(ctx, tx, userID, action, entityType, entityID, metadata); err != nil {
		log.Printf("audit: recording %s: %v", action, err)
	}
}
