package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/store"
)

// GetTieringPolicy returns the tenant's storage tiering policy.
func GetTieringPolicy(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		policy, err := store.GetTieringPolicy(ctx, tx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading tiering policy: "+err.Error())
			return
		}

		Success(c, gin.H{"policy": policy})
	}
}

// UpdateTieringPolicy updates the tenant's storage tiering policy.
func UpdateTieringPolicy(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var req struct {
			Enabled           bool   `json:"enabled"`
			TierDownAfterDays int    `json:"tierDownAfterDays"`
			TierUpOnAccess    bool   `json:"tierUpOnAccess"`
			DefaultTier       string `json:"defaultTier"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.TierDownAfterDays < 0 {
			Err(c, http.StatusBadRequest, "tierDownAfterDays must be >= 0")
			return
		}

		validTiers := map[string]bool{"standard": true, "infrequent": true, "archive": true}
		if req.DefaultTier != "" && !validTiers[req.DefaultTier] {
			Err(c, http.StatusBadRequest, "defaultTier must be one of: standard, infrequent, archive")
			return
		}

		policy, err := store.GetTieringPolicy(ctx, tx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading tiering policy: "+err.Error())
			return
		}

		policy.Enabled = req.Enabled
		if req.TierDownAfterDays > 0 {
			policy.TierDownAfterDays = req.TierDownAfterDays
		}
		policy.TierUpOnAccess = req.TierUpOnAccess
		if req.DefaultTier != "" {
			policy.DefaultTier = req.DefaultTier
		}

		if policy.ID == uuid.Nil {
			policy.ID = uuid.New()
		}

		if err := store.SaveTieringPolicy(ctx, tx, policy); err != nil {
			Err(c, http.StatusInternalServerError, "saving tiering policy: "+err.Error())
			return
		}

		Success(c, gin.H{"policy": policy})
	}
}

// RunTiering manually triggers the storage tiering job.
func RunTiering(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		policy, err := store.GetTieringPolicy(ctx, tx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading tiering policy: "+err.Error())
			return
		}

		tiered, err := store.RunStorageTiering(ctx, tx, policy)
		if err != nil {
			Err(c, http.StatusInternalServerError, "running tiering: "+err.Error())
			return
		}

		Success(c, gin.H{"tiered": tiered})
	}
}

// SetFileTier manually overrides a file's storage tier.
func SetFileTier(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid file id")
			return
		}

		var req struct {
			Tier string `json:"tier" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "tier is required")
			return
		}

		validTiers := map[string]bool{"standard": true, "infrequent": true, "archive": true}
		if !validTiers[req.Tier] {
			Err(c, http.StatusBadRequest, "tier must be one of: standard, infrequent, archive")
			return
		}

		var f model.File
		if err := tx.NewSelect().Model(&f).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "file not found")
			return
		}

		if _, err := tx.NewUpdate().Model((*model.File)(nil)).
			Set("storage_tier = ?", req.Tier).
			Set("updated_at = ?", time.Now()).
			Where("id = ?", id).
			Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating file tier: "+err.Error())
			return
		}

		Success(c, gin.H{"file_id": id, "tier": req.Tier})
	}
}
