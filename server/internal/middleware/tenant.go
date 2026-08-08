package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"

	"go-drive/server/internal/model"
	"go-drive/server/internal/tenant"
)

// Tenant returns middleware that resolves the tenant from the X-Org-Slug header,
// validates user membership, and sets the Postgres search_path to the tenant
// schema within a transaction. The transaction is stored in the context as
// "tenant_tx" for downstream handlers. On SQLite each tenant has its own
// database file; the transaction runs against that file (file-per-tenant).
func Tenant(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		orgSlug := c.GetHeader("X-Org-Slug")
		if orgSlug == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "X-Org-Slug header is required"})
			return
		}

		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		// Verify user is a member of this organization
		var member model.OrganizationMember
		err := db.NewSelect().
			Model(&member).
			Relation("Organization").
			Where("organization_member.user_id = ?", userID).
			Where("organization.slug = ?", orgSlug).
			Scan(ctx)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not a member of this organization"})
			return
		}

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
			tx, err = db.BeginTx(ctx, nil)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}

			schemaName := "tenant_" + orgSlug
			query := fmt.Sprintf("SET LOCAL search_path TO %s, public", pq.QuoteIdentifier(schemaName))
			_, err = tx.ExecContext(ctx, query)
			if err != nil {
				tx.Rollback()
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to set tenant context"})
				return
			}
		}

		c.Set("tenant_tx", tx)
		c.Set("org_id", member.OrganizationID.String())
		c.Set("org_slug", orgSlug)
		c.Set("org_role", member.Role)
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
