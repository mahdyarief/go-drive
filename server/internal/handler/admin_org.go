package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/migrate"
	"go-drive/server/internal/model"
	"go-drive/server/internal/tenant"
)

// AdminListOrgs returns all organizations for the admin management page.
func AdminListOrgs(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		type orgWithCount struct {
			model.Organization
			MemberCount int `bun:"member_count"`
		}
		var rows []orgWithCount
		err := db.NewRaw(`
			SELECT o.id, o.name, o.slug, o.created_at, o.updated_at, COUNT(m.id) AS member_count
			FROM organizations o
			LEFT JOIN organization_members m ON m.organization_id = o.id
			GROUP BY o.id, o.name, o.slug, o.created_at, o.updated_at
			ORDER BY o.name ASC
		`).Scan(ctx, &rows)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to list organizations")
			return
		}
		type orgView struct {
			ID            uuid.UUID `json:"id"`
			Name          string    `json:"name"`
			Slug          string    `json:"slug"`
			CreatedAt     time.Time `json:"created_at"`
			MemberCount   int       `json:"member_count"`
			StoreCount    int       `json:"store_count"`
			GDriveCount   int       `json:"gdrive_store_count"`
			StoreCapacity int64     `json:"store_capacity"`
			AttachedQuota int64     `json:"attached_quota"`
		}
		views := make([]orgView, 0, len(rows))
		for _, r := range rows {
			v := orgView{
				ID:          r.ID,
				Name:        r.Name,
				Slug:        r.Slug,
				CreatedAt:   r.CreatedAt,
				MemberCount: r.MemberCount,
			}

			// Stores live in the tenant schema — open a tenant tx per org
			// (works for both Postgres search_path and SQLite file-per-tenant).
			if tx, err := tenant.OpenTx(ctx, db, r.Slug); err == nil {
				type storeSummary struct {
					StoreCount    int   `bun:"store_count"`
					GDriveCount   int   `bun:"gdrive_count"`
					StoreCapacity int64 `bun:"store_capacity"`
				}
				var ss storeSummary
				if err := tx.NewRaw(`
					SELECT
						COUNT(CASE WHEN status = 'active' THEN 1 END) AS store_count,
						COUNT(CASE WHEN status = 'active' AND provider = 'gdrive' THEN 1 END) AS gdrive_count,
						COALESCE(SUM(CASE WHEN status = 'active' AND provider != 'gdrive' THEN quota_limit ELSE 0 END), 0) AS store_capacity
					FROM stores
				`).Scan(ctx, &ss); err == nil {
					v.StoreCount = ss.StoreCount
					v.GDriveCount = ss.GDriveCount
					v.StoreCapacity = ss.StoreCapacity
				}
				tx.Rollback()
			}

			if q, err := orgQuotaLimit(ctx, db, r.ID.String()); err == nil {
				v.AttachedQuota = q
			}

			views = append(views, v)
		}
		Success(c, gin.H{"orgs": views})
	}
}

// AdminGetOrg returns an organization's details and its members for admin
// management. Admin-scoped — bypasses membership checks.
func AdminGetOrg(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		ctx := c.Request.Context()

		var org model.Organization
		if err := db.NewSelect().Model(&org).Where("slug = ?", slug).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "organization not found")
			return
		}

		type memberView struct {
			ID        string    `json:"id"`
			UserID    string    `json:"user_id"`
			Name      string    `json:"name"`
			Email     string    `json:"email"`
			Role      string    `json:"role"`
			CreatedAt time.Time `json:"created_at"`
		}
		var members []memberView
		err := db.NewRaw(`
			SELECT CAST(m.id AS TEXT) AS id, m.user_id, u.name, u.email, m.role, m.created_at
			FROM organization_members m
			JOIN users u ON CAST(u.id AS TEXT) = m.user_id
			WHERE m.organization_id = ?
			ORDER BY m.created_at ASC
		`, org.ID).Scan(ctx, &members)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to list members")
			return
		}

		Success(c, gin.H{
			"org": gin.H{
				"id":         org.ID,
				"name":       org.Name,
				"slug":       org.Slug,
				"created_at": org.CreatedAt,
			},
			"members": members,
		})
	}
}

// AdminUpdateOrg renames an organization. Admin-scoped.
func AdminUpdateOrg(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		var req struct {
			Name string `json:"name" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "name is required")
			return
		}

		ctx := c.Request.Context()
		result, err := db.NewUpdate().
			Model((*model.Organization)(nil)).
			Set("name = ?", req.Name).
			Where("slug = ?", slug).
			Exec(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to update organization")
			return
		}
		if n, _ := result.RowsAffected(); n == 0 {
			Err(c, http.StatusNotFound, "organization not found")
			return
		}

		Msg(c, "organization updated")
	}
}

// AdminDeleteOrg deletes an organization and its tenant schema. Admin-scoped —
// mirrors DeleteOrg without the owner check.
func AdminDeleteOrg(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		ctx := c.Request.Context()

		var org model.Organization
		if err := db.NewSelect().Model(&org).Where("slug = ?", slug).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "organization not found")
			return
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			Err(c, http.StatusInternalServerError, "internal error")
			return
		}
		defer tx.Rollback()

		if err := migrate.DropTenantSchema(ctx, tx, slug); err != nil {
			Err(c, http.StatusInternalServerError, "failed to drop tenant schema")
			return
		}

		_, err = tx.NewDelete().
			Model((*model.Organization)(nil)).
			Where("id = ?", org.ID).
			Exec(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to delete organization")
			return
		}

		if err := tx.Commit(); err != nil {
			Err(c, http.StatusInternalServerError, "internal error")
			return
		}

		Msg(c, "organization deleted")
	}
}
