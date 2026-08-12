package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"

	"go-drive/server/internal/model"
	"go-drive/server/internal/tenant"
)

// hashAPIKey returns the SHA-256 hex digest of a raw API key. Only the hash
// is ever persisted or looked up — the raw secret never touches the database.
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// RequireAPIKey authenticates requests via an `Authorization: Bearer <key>`
// header against the public-schema api_keys table. The key is matched by its
// SHA-256 hash so the raw secret is never stored. On success it sets user_id,
// org_slug, and api_key_id in the gin context. A best-effort last_used_at
// update runs afterwards and never fails the request.
func RequireAPIKey(db *bun.DB, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			return
		}
		rawKey := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if rawKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			return
		}

		ctx := c.Request.Context()
		var key model.APIKey
		err := db.NewSelect().Model(&key).Where("key_hash = ?", hashAPIKey(rawKey)).Scan(ctx)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}
		if key.Status != "active" || key.RevokedAt != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key is revoked or inactive"})
			return
		}
		if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key has expired"})
			return
		}

		hasScope := false
		for _, s := range key.Scopes {
			if s == scope {
				hasScope = true
				break
			}
		}
		if !hasScope {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key does not have the required scope"})
			return
		}

		c.Set("user_id", key.UserID)
		c.Set("org_slug", key.OrgSlug)
		c.Set("api_key_id", key.ID.String())

		// Best-effort last_used_at update — never fail the request on it.
		now := time.Now()
		_, _ = db.NewUpdate().Model((*model.APIKey)(nil)).Set("last_used_at = ?", now).Where("id = ?", key.ID).Exec(ctx)

		c.Next()
	}
}

// APIKeyTenantTx opens a tenant transaction for the org bound to the API key
// (set in the context by RequireAPIKey) and stores it as "tenant_tx", so the
// handler can run the same store pipeline as the session-authenticated upload.
// It mirrors middleware.Tenant's tx bootstrap (SQLite: file-per-tenant DB;
// Postgres: SET LOCAL search_path) but skips the membership check and header —
// the org comes from the key itself. Must run AFTER RequireAPIKey.
func APIKeyTenantTx(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgSlug := c.GetString("org_slug")
		if orgSlug == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key is not bound to an organization"})
			return
		}

		ctx := c.Request.Context()
		var tx bun.Tx
		if db.Dialect().Name() == dialect.SQLite {
			// File-per-tenant: run the request against the tenant's own DB file.
			tdb, err := tenant.DB(ctx, orgSlug)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to open tenant database"})
				return
			}
			tx, err = tdb.BeginTx(ctx, nil)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
		} else {
			// Postgres: begin transaction and set search_path to tenant schema.
			var err error
			tx, err = db.BeginTx(ctx, nil)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
			schemaName := "tenant_" + orgSlug
			query := fmt.Sprintf("SET LOCAL search_path TO %s, public", pq.QuoteIdentifier(schemaName))
			if _, err := tx.ExecContext(ctx, query); err != nil {
				tx.Rollback()
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to set tenant context"})
				return
			}
		}

		c.Set("tenant_tx", tx)
		c.Set("db", db)

		c.Next()

		// Commit or rollback based on response status
		if c.IsAborted() || c.Writer.Status() >= 400 {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}
}
