package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/store"
)

// ListAuditLogs returns the current user's recent audit entries (newest first).
func ListAuditLogs(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		p := ParsePagination(c)

		q := tx.NewSelect().Model((*model.AuditLog)(nil)).
			Where("user_id = ?", userID).
			Order("created_at DESC")

		total, err := q.Count(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "counting audit logs: "+err.Error())
			return
		}

		var logs []model.AuditLog
		if err := q.Limit(p.PageSize).Offset(p.Offset).Scan(ctx, &logs); err != nil {
			Err(c, http.StatusInternalServerError, "listing audit logs: "+err.Error())
			return
		}
		PaginatedResponse(c, "logs", logs, total, p)
	}
}

// auditLog records a best-effort audit entry; failures are logged and never
// fail the main operation.
func auditLog(ctx context.Context, tx bun.IDB, userID, action, entityType, entityID string, metadata map[string]any) {
	if err := store.CreateAuditLog(ctx, tx, userID, action, entityType, entityID, metadata); err != nil {
		log.Printf("audit: recording %s: %v", action, err)
	}
}
