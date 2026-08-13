package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/tenant"
)

// userQuotaLimit returns the admin-assigned storage limit for a user
// (0 = unlimited / no row).
func userQuotaLimit(ctx context.Context, db *bun.DB, userID string) (int64, error) {
	var q model.UserQuota
	err := db.NewSelect().Model(&q).Where("user_id = ?", userID).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0, nil
		}
		return 0, err
	}
	return q.QuotaLimit, nil
}

// userQuotaAllocated returns the sum of org quotas owned by a user.
func userQuotaAllocated(ctx context.Context, db *bun.DB, userID string) (int64, error) {
	var allocated int64
	err := db.NewRaw(
		"SELECT COALESCE(SUM(quota_limit), 0) FROM org_quotas WHERE owner_user_id = ?",
		userID,
	).Scan(ctx, &allocated)
	return allocated, err
}

// orgQuotaLimit returns the quota allocated to an org (0 = unlimited / no row).
func orgQuotaLimit(ctx context.Context, db *bun.DB, orgID string) (int64, error) {
	var q model.OrgQuota
	err := db.NewSelect().Model(&q).Where("organization_id = ?", orgID).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0, nil
		}
		return 0, err
	}
	return q.QuotaLimit, nil
}

// orgOwnerUserLimit returns the admin-assigned storage limit of the user who
// owns the org (0 = unlimited / no row). This caps the org's effective storage
// even when no explicit org allocation has been set.
func orgOwnerUserLimit(ctx context.Context, db *bun.DB, orgSlug string) (int64, error) {
	var limit int64
	err := db.NewRaw(`
		SELECT COALESCE(uq.quota_limit, 0)
		FROM organization_members om
		JOIN organizations o ON o.id = om.organization_id
		LEFT JOIN user_quotas uq ON uq.user_id = om.user_id
		WHERE o.slug = ? AND om.role = 'owner'
		LIMIT 1
	`, orgSlug).Scan(ctx, &limit)
	if err != nil {
		return 0, err
	}
	return limit, nil
}

// SetOrgQuota sets the storage allocation an owner assigns to one of their
// orgs. Validates the total of the user's org quotas stays within their
// admin-assigned limit (0 = unlimited).
func SetOrgQuota(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		slug := c.Param("slug")
		ctx := c.Request.Context()

		var req struct {
			Limit int64 `json:"limit"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request")
			return
		}
		if req.Limit < 0 {
			Err(c, http.StatusBadRequest, "limit must not be negative")
			return
		}

		var member model.OrganizationMember
		err := db.NewSelect().
			Model(&member).
			Relation("Organization").
			Where("organization_member.user_id = ?", userID).
			Where("organization.slug = ?", slug).
			Scan(ctx)
		if err != nil {
			Err(c, http.StatusNotFound, "organization not found")
			return
		}
		if member.Role != "owner" {
			Err(c, http.StatusForbidden, "only the owner can set the storage quota")
			return
		}

		limit, err := userQuotaLimit(ctx, db, userID)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to read user quota")
			return
		}
		allocated, err := userQuotaAllocated(ctx, db, userID)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to read allocated quota")
			return
		}
		// The current org's old allocation is being replaced, so exclude it
		// from the "already allocated" total before validating the new sum.
		other := allocated
		if old, err := orgQuotaLimit(ctx, db, member.OrganizationID.String()); err == nil {
			other -= old
		}
		if limit > 0 && other+req.Limit > limit {
			Err(c, http.StatusBadRequest, "total org allocations exceed the user's storage limit")
			return
		}

		orgQuota := &model.OrgQuota{
			OrganizationID: member.OrganizationID,
			OwnerUserID:    userID,
			QuotaLimit:     req.Limit,
			UpdatedAt:      time.Now(),
		}
		_, err = db.NewInsert().
			Model(orgQuota).
			On("CONFLICT (organization_id) DO UPDATE SET owner_user_id = EXCLUDED.owner_user_id, quota_limit = EXCLUDED.quota_limit, updated_at = EXCLUDED.updated_at").
			Exec(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to set org quota")
			return
		}

		Success(c, gin.H{"organization_id": member.OrganizationID, "limit": req.Limit})
	}
}

// AdminSetUserLimit sets the admin-assigned storage limit for a user.
// Cannot be lowered below the sum of the user's current org allocations.
func AdminSetUserLimit(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("id")
		ctx := c.Request.Context()

		var req struct {
			Limit int64 `json:"limit"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request")
			return
		}
		if req.Limit < 0 {
			Err(c, http.StatusBadRequest, "limit must not be negative")
			return
		}

		var count int
		if err := db.NewRaw("SELECT COUNT(*) FROM users WHERE id = ?", userID).Scan(ctx, &count); err != nil || count == 0 {
			Err(c, http.StatusNotFound, "user not found")
			return
		}

		allocated, err := userQuotaAllocated(ctx, db, userID)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to read allocated quota")
			return
		}
		if req.Limit > 0 && allocated > req.Limit {
			Err(c, http.StatusBadRequest, "limit is below the user's current org allocations")
			return
		}

		quota := &model.UserQuota{
			UserID:     userID,
			QuotaLimit: req.Limit,
			UpdatedBy:  c.GetString("user_id"),
			UpdatedAt:  time.Now(),
		}
		_, err = db.NewInsert().
			Model(quota).
			On("CONFLICT (user_id) DO UPDATE SET quota_limit = EXCLUDED.quota_limit, updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at").
			Exec(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to set user quota")
			return
		}

		Success(c, gin.H{"user_id": userID, "limit": req.Limit})
	}
}

// AdminOrgStorage returns an org's attached stores and its quota allocation
// for the admin organization detail view.
func AdminOrgStorage(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("slug")
		ctx := c.Request.Context()

		var org model.Organization
		if err := db.NewSelect().Model(&org).Where("slug = ?", slug).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "organization not found")
			return
		}

		tx, err := tenant.OpenTx(ctx, db, slug)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to open tenant schema")
			return
		}
		defer tx.Rollback()

		var stores []model.Store
		if err := tx.NewSelect().Model(&stores).Order("created_at ASC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "failed to list stores")
			return
		}

		allocated, err := orgQuotaLimit(ctx, db, org.ID.String())
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to read org quota")
			return
		}

		Success(c, gin.H{
			"organization": gin.H{"id": org.ID, "name": org.Name, "slug": org.Slug},
			"quota_limit":  allocated,
			"stores":       stores,
		})
	}
}

// checkOrgUploadQuota rejects an upload (caller maps this to 413) when the
// org's allocated quota for local provider storage would be exceeded.
// GDrive stores are exempt — their capacity is attached per-org and not
// counted against the user's quota.
func checkOrgUploadQuota(ctx context.Context, db *bun.DB, tx bun.Tx, orgSlug string, totalSize int64) error {
	if orgSlug == "" || totalSize <= 0 {
		return nil
	}
	var limit int64
	if err := db.NewRaw(`
		SELECT COALESCE(oq.quota_limit, 0)
		FROM org_quotas oq
		JOIN organizations o ON o.id = oq.organization_id
		WHERE o.slug = ?
	`, orgSlug).Scan(ctx, &limit); err != nil {
		limit = 0 // no allocation row → fall back to the owner's user limit
	}
	// The owner's admin-assigned limit caps the org's storage even when no
	// explicit org allocation row exists.
	if ownerLimit, err := orgOwnerUserLimit(ctx, db, orgSlug); err == nil && ownerLimit > 0 && (limit == 0 || ownerLimit < limit) {
		limit = ownerLimit
	}
	if limit <= 0 {
		return nil // unlimited
	}

	var used int64
	if err := tx.NewRaw(`
		SELECT COALESCE(SUM(f.size), 0)
		FROM files f
		WHERE f.status = 'ready'
		  AND EXISTS (
			SELECT 1 FROM blob_locations bl
			JOIN stores s ON s.id = bl.store_id
			WHERE bl.blob_id = f.blob_id AND s.provider = 'local'
		  )
	`).Scan(ctx, &used); err != nil {
		return err
	}
	if used+totalSize > limit {
		return errors.New("storage quota exceeded")
	}
	return nil
}
