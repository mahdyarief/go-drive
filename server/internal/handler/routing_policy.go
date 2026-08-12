package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// GetRoutingPolicy returns the workspace's upload routing policy. When no row
// exists the default policy (most_available, empty priority list, cursor 0)
// is returned without creating one.
func GetRoutingPolicy(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		policy, err := loadRoutingPolicy(ctx, tx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading routing policy: "+err.Error())
			return
		}
		Success(c, gin.H{"policy": policy})
	}
}

// UpdateRoutingPolicy upserts the workspace's upload routing policy.
// Body: { mode?, priority_store_ids?, round_robin_cursor? }. Only
// round_robin_cursor value accepted is 0 (reset).
func UpdateRoutingPolicy(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var req struct {
			Mode             *string     `json:"mode"`
			PriorityStoreIDs []uuid.UUID `json:"priority_store_ids"`
			RoundRobinCursor *int        `json:"round_robin_cursor"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}

		policy, err := loadRoutingPolicy(ctx, tx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading routing policy: "+err.Error())
			return
		}

		if req.Mode != nil {
			mode := strings.TrimSpace(*req.Mode)
			if mode != "most_available" && mode != "round_robin" && mode != "priority" {
				Err(c, http.StatusBadRequest, "mode must be 'most_available', 'round_robin', or 'priority'")
				return
			}
			policy.Mode = mode
		}
		if req.PriorityStoreIDs != nil {
			policy.PriorityStoreIDs = req.PriorityStoreIDs
		}
		if req.RoundRobinCursor != nil {
			if *req.RoundRobinCursor != 0 {
				Err(c, http.StatusBadRequest, "round_robin_cursor can only be reset to 0")
				return
			}
			policy.RoundRobinCursor = 0
		}

		policy.WorkspaceID = uuid.Nil
		policy.UpdatedAt = time.Now()
		if _, err := tx.NewInsert().Model(policy).
			On("CONFLICT (workspace_id) DO UPDATE").
			Set("mode = EXCLUDED.mode").
			Set("priority_store_ids = EXCLUDED.priority_store_ids").
			Set("round_robin_cursor = EXCLUDED.round_robin_cursor").
			Set("updated_at = EXCLUDED.updated_at").
			Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "saving routing policy: "+err.Error())
			return
		}

		Success(c, gin.H{"policy": policy})
	}
}

// loadRoutingPolicy returns the workspace's routing policy row, or a default
// policy (most_available) when no row exists yet. Reads never create a row.
func loadRoutingPolicy(ctx context.Context, tx bun.IDB) (*model.StoreRoutingPolicy, error) {
	var policy model.StoreRoutingPolicy
	err := tx.NewSelect().Model(&policy).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &model.StoreRoutingPolicy{
				WorkspaceID:      uuid.Nil,
				Mode:             "most_available",
				PriorityStoreIDs: []uuid.UUID{},
			}, nil
		}
		return nil, err
	}
	if policy.PriorityStoreIDs == nil {
		policy.PriorityStoreIDs = []uuid.UUID{}
	}
	return &policy, nil
}
