package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

const (
	apiKeySecretPrefix = "9d_live_"
	apiKeyDefaultScope = "files:upload"
)

// validAPIKeyScopes is the set of scopes a key may be granted.
var validAPIKeyScopes = map[string]bool{
	"files:upload": true,
}

// ListAPIKeys returns the API keys for the current org. The table lives in
// the PUBLIC schema, so it is queried with the shared db handle, NOT
// tenant_tx. key_hash is never returned.
func ListAPIKeys(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgSlug := c.GetString("org_slug")
		ctx := c.Request.Context()
		p := ParsePagination(c)

		q := db.NewSelect().Model((*model.APIKey)(nil)).Where("org_slug = ?", orgSlug).Order("created_at DESC")
		total, err := q.Count(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "counting API keys: "+err.Error())
			return
		}
		var keys []model.APIKey
		if err := q.Limit(p.PageSize).Offset(p.Offset).Scan(ctx, &keys); err != nil {
			Err(c, http.StatusInternalServerError, "listing API keys: "+err.Error())
			return
		}
		for i := range keys {
			keys[i].KeyHash = ""
		}
		PaginatedResponse(c, "keys", keys, total, p)
	}
}

// CreateAPIKey generates a new API key for the current org. Body:
// { name, scopes? } (scopes defaults to ["files:upload"]). The full secret is
// returned exactly once in the response and never stored — only its SHA-256
// hash is persisted.
func CreateAPIKey(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgSlug := c.GetString("org_slug")
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		var req struct {
			Name   string   `json:"name"`
			Scopes []string `json:"scopes"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		req.Name = trimSpace(req.Name)
		if req.Name == "" {
			Err(c, http.StatusBadRequest, "name is required")
			return
		}
		if len(req.Scopes) == 0 {
			req.Scopes = []string{apiKeyDefaultScope}
		}
		for _, s := range req.Scopes {
			if !validAPIKeyScopes[s] {
				Err(c, http.StatusBadRequest, "invalid scope: "+s)
				return
			}
		}

		secret := apiKeySecretPrefix + tokenHex(20) // 40 hex chars
		sum := sha256.Sum256([]byte(secret))

		k := &model.APIKey{
			ID:        uuid.New(),
			OrgSlug:   orgSlug,
			UserID:    userID,
			Name:      req.Name,
			KeyPrefix: secret[:16],
			KeyHash:   hex.EncodeToString(sum[:]),
			Scopes:    req.Scopes,
			Status:    "active",
		}
		if _, err := db.NewInsert().Model(k).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "creating API key: "+err.Error())
			return
		}
		k.KeyHash = ""
		tx := c.MustGet("tenant_tx").(bun.Tx)
		auditLog(ctx, tx, userID, "api_key_create", "api_key", k.ID.String(), map[string]any{"name": k.Name})
		Created(c, gin.H{"key": k, "secret": secret})
	}
}

// DeleteAPIKey soft-deletes an API key: status -> 'revoked' + revoked_at.
// Scoped to the current org so tenants cannot touch each other's keys.
func DeleteAPIKey(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgSlug := c.GetString("org_slug")
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid API key id")
			return
		}
		var k model.APIKey
		if err := db.NewSelect().Model(&k).Where("id = ?", id).Where("org_slug = ?", orgSlug).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "API key not found")
			return
		}
		now := time.Now()
		res, err := db.NewUpdate().Model((*model.APIKey)(nil)).
			Set("status = 'revoked', revoked_at = ?", now).
			Where("id = ?", id).
			Where("org_slug = ?", orgSlug).
			Exec(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "revoking API key: "+err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			Err(c, http.StatusNotFound, "API key not found")
			return
		}
		tx := c.MustGet("tenant_tx").(bun.Tx)
		auditLog(ctx, tx, userID, "api_key_revoke", "api_key", id.String(), map[string]any{"name": k.Name})
		Msg(c, "API key revoked")
	}
}
