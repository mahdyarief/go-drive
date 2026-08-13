package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/security"
	"go-drive/server/internal/storage"
	"go-drive/server/internal/store"
	"go-drive/server/internal/tenant"
)

// ListTrackedLinks returns all tracked links in the workspace.
func ListTrackedLinks(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var links []model.TrackedLink
		if err := tx.NewSelect().Model(&links).Order("created_at DESC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing tracked links: "+err.Error())
			return
		}
		Success(c, gin.H{"links": links})
	}
}

// CreateTrackedLink creates a tracked link. Body: { fileId?, folderId?, name,
// description?, access?, password?, requireEmail?, expiresAt?, validFrom?,
// validUntil?, maxViews? }. Exactly one of fileId/folderId.
func CreateTrackedLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		orgSlug := c.GetString("org_slug")
		ctx := c.Request.Context()

		var req struct {
			FileID       *uuid.UUID `json:"fileId"`
			FolderID     *uuid.UUID `json:"folderId"`
			Name         string     `json:"name"`
			Description  string     `json:"description"`
			Access       string     `json:"access"`
			Password     string     `json:"password"`
			RequireEmail bool       `json:"requireEmail"`
			ExpiresAt    *time.Time `json:"expiresAt"`
			ValidFrom    *time.Time `json:"validFrom"`
			ValidUntil   *time.Time `json:"validUntil"`
			MaxViews     *int       `json:"maxViews"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if (req.FileID == nil) == (req.FolderID == nil) {
			Err(c, http.StatusBadRequest, "exactly one of fileId or folderId is required")
			return
		}
		req.Name = trimSpace(req.Name)
		if req.Name == "" {
			Err(c, http.StatusBadRequest, "name is required")
			return
		}
		if req.FileID != nil {
			if err := fileReadyExists(ctx, tx, *req.FileID); err != nil {
				Err(c, http.StatusNotFound, "file not found")
				return
			}
		} else if err := folderExists(ctx, tx, *req.FolderID); err != nil {
			Err(c, http.StatusNotFound, "folder not found")
			return
		}
		access := req.Access
		if access == "" {
			access = "view"
		}
		if access != "view" && access != "download" {
			Err(c, http.StatusBadRequest, "access must be view or download")
			return
		}
		hash, err := security.HashLinkPassword(req.Password)
		if err != nil {
			Err(c, http.StatusInternalServerError, "hashing password: "+err.Error())
			return
		}
		token, err := security.TokenHex16()
		if err != nil {
			Err(c, http.StatusInternalServerError, "generating token: "+err.Error())
			return
		}

		link := &model.TrackedLink{
			ID:           uuid.New(),
			UserID:       userID,
			FileID:       req.FileID,
			FolderID:     req.FolderID,
			Token:        token,
			Name:         req.Name,
			Description:  req.Description,
			Access:       access,
			HasPassword:  req.Password != "",
			PasswordHash: hash,
			RequireEmail: req.RequireEmail,
			ExpiresAt:    req.ExpiresAt,
			ValidFrom:    req.ValidFrom,
			ValidUntil:   req.ValidUntil,
			MaxViews:     req.MaxViews,
			IsActive:     true,
		}
		if _, err := tx.NewInsert().Model(link).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "creating tracked link: "+err.Error())
			return
		}
		if err := registerLinkToken(ctx, db, token, orgSlug, "tracked", link.ID.String()); err != nil {
			Err(c, http.StatusInternalServerError, "registering link token: "+err.Error())
			return
		}
		Created(c, gin.H{"link": link})
	}
}

// UpdateTrackedLink updates a tracked link. Body: { name?, description?,
// access?, password?, requireEmail?, expiresAt?, validFrom?, validUntil?,
// maxViews?, isActive? }.
func UpdateTrackedLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid tracked link id")
			return
		}
		var req struct {
			Name         *string    `json:"name"`
			Description  *string    `json:"description"`
			Access       *string    `json:"access"`
			Password     *string    `json:"password"`
			RequireEmail *bool      `json:"requireEmail"`
			ExpiresAt    *time.Time `json:"expiresAt"`
			ValidFrom    *time.Time `json:"validFrom"`
			ValidUntil   *time.Time `json:"validUntil"`
			MaxViews     *int       `json:"maxViews"`
			IsActive     *bool      `json:"isActive"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}

		var link model.TrackedLink
		if err := tx.NewSelect().Model(&link).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "tracked link not found")
			return
		}
		u := tx.NewUpdate().Model((*model.TrackedLink)(nil))
		if req.Name != nil {
			if n := trimSpace(*req.Name); n != "" {
				u.Set("name = ?", n)
			}
		}
		if req.Description != nil {
			u.Set("description = ?", *req.Description)
		}
		if req.Access != nil && *req.Access != "" && *req.Access != link.Access {
			if *req.Access != "view" && *req.Access != "download" {
				Err(c, http.StatusBadRequest, "access must be view or download")
				return
			}
			u.Set("access = ?", *req.Access)
		}
		if req.Password != nil {
			hash, err := security.HashLinkPassword(*req.Password)
			if err != nil {
				Err(c, http.StatusInternalServerError, "hashing password: "+err.Error())
				return
			}
			u.Set("has_password = ?, password_hash = ?", *req.Password != "", hash)
		}
		if req.RequireEmail != nil {
			u.Set("require_email = ?", *req.RequireEmail)
		}
		if req.ExpiresAt != nil {
			u.Set("expires_at = ?", *req.ExpiresAt)
		}
		if req.ValidFrom != nil {
			u.Set("valid_from = ?", *req.ValidFrom)
		}
		if req.ValidUntil != nil {
			u.Set("valid_until = ?", *req.ValidUntil)
		}
		if req.MaxViews != nil {
			u.Set("max_views = ?", *req.MaxViews)
		}
		if req.IsActive != nil {
			u.Set("is_active = ?", *req.IsActive)
		}
		u.Set("updated_at = ?", time.Now())
		if _, err := u.Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating tracked link: "+err.Error())
			return
		}
		var updated model.TrackedLink
		if err := tx.NewSelect().Model(&updated).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "reloading tracked link: "+err.Error())
			return
		}
		Success(c, gin.H{"link": updated})
	}
}

// DeleteTrackedLink removes a tracked link, its events, and its registry token.
func DeleteTrackedLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid tracked link id")
			return
		}
		var link model.TrackedLink
		if err := tx.NewSelect().Model(&link).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "tracked link not found")
			return
		}
		if _, err := tx.NewDelete().Model((*model.TrackedLinkEvent)(nil)).Where("tracked_link_id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "clearing events: "+err.Error())
			return
		}
		if _, err := tx.NewDelete().Model((*model.TrackedLink)(nil)).Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "deleting tracked link: "+err.Error())
			return
		}
		_ = deleteLinkToken(ctx, db, link.Token)
		Msg(c, "tracked link deleted")
	}
}

// ListTrackedLinkEvents returns the analytics events for a tracked link.
func ListTrackedLinkEvents(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid tracked link id")
			return
		}
		var events []model.TrackedLinkEvent
		if err := tx.NewSelect().Model(&events).
			Where("tracked_link_id = ?", id).
			Order("timestamp DESC").
			Limit(200).
			Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing events: "+err.Error())
			return
		}
		Success(c, gin.H{"events": events})
	}
}

// PublicTrackedLink serves GET /api/tracked/:token — records a 'view' event
// and returns a file/folder browse payload.
func PublicTrackedLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tx, link, ok := openTrackedLink(c, db, linkPassword(c), false)
		if !ok {
			return
		}
		defer finishPublicTx(c, tx)

		if err := recordTrackedView(ctx, tx, link.ID, c); err != nil {
			Err(c, http.StatusInternalServerError, "recording view: "+err.Error())
			return
		}
		if _, err := tx.NewUpdate().Model((*model.TrackedLink)(nil)).
			Set("view_count = view_count + 1, last_accessed_at = ?", time.Now()).
			Where("id = ?", link.ID).
			Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating view count: "+err.Error())
			return
		}

		if link.FileID != nil {
			var f model.File
			if err := tx.NewSelect().Model(&f).Where("id = ? AND status = 'ready'", *link.FileID).Scan(ctx); err != nil {
				Err(c, http.StatusNotFound, "file not found")
				return
			}
			Success(c, gin.H{
				"type":     "file",
				"name":     f.Name,
				"mimeType": f.MimeType,
				"size":     f.Size,
			})
			return
		}

		var folders []model.Folder
		if err := tx.NewSelect().Model(&folders).
			Where("parent_id = ? AND name NOT LIKE '.%'", *link.FolderID).
			Order("name ASC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing folders: "+err.Error())
			return
		}
		var files []model.File
		if err := tx.NewSelect().Model(&files).
			Where("folder_id = ? AND status = 'ready' AND name NOT LIKE '.%'", *link.FolderID).
			Order("name ASC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing files: "+err.Error())
			return
		}
		Success(c, gin.H{"type": "folder", "folders": folders, "files": files})
	}
}

// PublicTrackedDownload serves GET /api/tracked/:token/download?fileId= —
// records a 'download' event and returns a signed URL for the file.
func PublicTrackedDownload(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tx, link, ok := openTrackedLink(c, db, linkPassword(c), true)
		if !ok {
			return
		}
		defer finishPublicTx(c, tx)

		f, ok := resolveSharedFile(c, tx, link.FileID, link.FolderID, c.Query("fileId"))
		if !ok {
			return
		}
		st, err := buildReadStorage(ctx, tx, f.BlobID, f.StoragePath)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		url, err := st.GetSignedURL(ctx, f.StoragePath, time.Hour)
		if err != nil {
			if errors.Is(err, storage.ErrNotSupported) {
				Err(c, http.StatusNotImplemented, "signed URLs not supported for this provider")
				return
			}
			Err(c, http.StatusInternalServerError, "signing url: "+err.Error())
			return
		}
		if err := recordTrackedDownload(ctx, tx, link.ID, c); err != nil {
			Err(c, http.StatusInternalServerError, "recording download: "+err.Error())
			return
		}
		if _, err := tx.NewUpdate().Model((*model.TrackedLink)(nil)).
			Set("download_count = download_count + 1, last_accessed_at = ?", time.Now()).
			Where("id = ?", link.ID).
			Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating download count: "+err.Error())
			return
		}
		Success(c, gin.H{
			"url":      url,
			"filename": f.Name,
			"mimeType": f.MimeType,
			"size":     f.Size,
		})
	}
}

// PublicTrackedRaw serves GET /api/tracked/:token/raw?fileId= — streams the
// file bytes directly and records a 'download' event.
func PublicTrackedRaw(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tx, link, ok := openTrackedLink(c, db, linkPassword(c), true)
		if !ok {
			return
		}
		defer finishPublicTx(c, tx)

		f, ok := resolveSharedFile(c, tx, link.FileID, link.FolderID, c.Query("fileId"))
		if !ok {
			return
		}
		st, err := buildReadStorage(ctx, tx, f.BlobID, f.StoragePath)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		r, size, err := st.Download(ctx, f.StoragePath)
		if err != nil {
			Err(c, http.StatusInternalServerError, "downloading file: "+err.Error())
			return
		}
		defer r.Close()
		mime := f.MimeType
		if mime == "" {
			mime = "application/octet-stream"
		}
		c.Header("Content-Disposition", `attachment; filename="`+f.Name+`"`)
		c.DataFromReader(http.StatusOK, size, mime, r, nil)
		_ = recordTrackedDownload(ctx, tx, link.ID, c)
		_, _ = tx.NewUpdate().Model((*model.TrackedLink)(nil)).
			Set("download_count = download_count + 1, last_accessed_at = ?", time.Now()).
			Where("id = ?", link.ID).
			Exec(ctx)
	}
}

// openTrackedLink resolves a tracked token, opens the tenant tx, loads and
// validates the link. whenDownload additionally enforces the 'view' access
// restriction.
func openTrackedLink(c *gin.Context, db *bun.DB, password string, whenDownload bool) (bun.Tx, *model.TrackedLink, bool) {
	ctx := c.Request.Context()
	row, err := resolveLinkToken(ctx, db, c.Param("token"))
	if err != nil {
		Err(c, http.StatusInternalServerError, "resolving token: "+err.Error())
		return bun.Tx{}, nil, false
	}
	if row == nil || row.LinkType != "tracked" {
		Err(c, http.StatusNotFound, "link not found")
		return bun.Tx{}, nil, false
	}
	linkID, err := uuid.Parse(row.LinkID)
	if err != nil {
		Err(c, http.StatusNotFound, "link not found")
		return bun.Tx{}, nil, false
	}
	tx, err := tenant.OpenTx(ctx, db, row.OrgSlug)
	if err != nil {
		Err(c, http.StatusInternalServerError, "opening tenant: "+err.Error())
		return bun.Tx{}, nil, false
	}
	ok := false
	defer func() {
		if !ok {
			_ = tx.Rollback()
		}
	}()

	var link model.TrackedLink
	if err := tx.NewSelect().Model(&link).Where("id = ?", linkID).Scan(ctx); err != nil {
		Err(c, http.StatusNotFound, "link not found")
		return bun.Tx{}, &link, false
	}
	if !link.IsActive {
		Err(c, http.StatusForbidden, "link is inactive")
		return bun.Tx{}, &link, false
	}
	now := time.Now()
	if link.ExpiresAt != nil && now.After(*link.ExpiresAt) {
		Err(c, http.StatusForbidden, "link has expired")
		return bun.Tx{}, &link, false
	}
	if link.ValidFrom != nil && now.Before(*link.ValidFrom) {
		Err(c, http.StatusForbidden, "link is not yet active")
		return bun.Tx{}, &link, false
	}
	if link.ValidUntil != nil && now.After(*link.ValidUntil) {
		Err(c, http.StatusForbidden, "link is no longer active")
		return bun.Tx{}, &link, false
	}
	if link.MaxViews != nil && link.ViewCount >= *link.MaxViews {
		Err(c, http.StatusForbidden, "view limit reached")
		return bun.Tx{}, &link, false
	}
	if link.Access == "view" && whenDownload {
		Err(c, http.StatusForbidden, "downloads are not enabled for this link")
		return bun.Tx{}, &link, false
	}
	if link.HasPassword && !security.VerifyLinkPassword(link.PasswordHash, password) {
		Err(c, http.StatusUnauthorized, "invalid password")
		return bun.Tx{}, &link, false
	}
	if link.RequireEmail && c.Query("email") == "" {
		Err(c, http.StatusBadRequest, "email is required for this link")
		return bun.Tx{}, &link, false
	}
	ok = true
	return tx, &link, true
}

// recordTrackedView inserts a view event for a tracked link.
func recordTrackedView(ctx context.Context, tx bun.Tx, linkID uuid.UUID, c *gin.Context) error {
	return insertTrackedEvent(ctx, tx, linkID, "view", c)
}

// recordTrackedDownload inserts a download event for a tracked link.
func recordTrackedDownload(ctx context.Context, tx bun.Tx, linkID uuid.UUID, c *gin.Context) error {
	return insertTrackedEvent(ctx, tx, linkID, "download", c)
}

// insertTrackedEvent builds and inserts a tracked_link_event row from the
// request context (IP, UA, referrer, UTM, language, email).
func insertTrackedEvent(ctx context.Context, tx bun.Tx, linkID uuid.UUID, eventType string, c *gin.Context) error {
	browser, osName, device := parseUA(c.GetHeader("User-Agent"))
	ev := &model.TrackedLinkEvent{
		ID:            uuid.New(),
		TrackedLinkID: linkID,
		EventType:     eventType,
		VisitorID:     c.Query("visitor"),
		Email:         c.Query("email"),
		IPAddress:     c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
		Browser:       browser,
		OS:            osName,
		DeviceType:    device,
		Referrer:      c.GetHeader("Referer"),
		UTMSource:     c.Query("utm_source"),
		UTMMedium:     c.Query("utm_medium"),
		UTMCampaign:   c.Query("utm_campaign"),
		Language:      c.GetHeader("Accept-Language"),
	}
	_, err := tx.NewInsert().Model(ev).Exec(ctx)
	return err
}

// parseUA does a lightweight browser/OS/device detection from a user agent.
func parseUA(ua string) (browser, osName, device string) {
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	case strings.Contains(ua, "MSIE") || strings.Contains(ua, "Trident/"):
		browser = "IE"
	}
	switch {
	case strings.Contains(ua, "Windows"):
		osName = "Windows"
	case strings.Contains(ua, "Mac OS X"):
		osName = "macOS"
	case strings.Contains(ua, "Android"):
		osName = "Android"
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad"):
		osName = "iOS"
	case strings.Contains(ua, "Linux"):
		osName = "Linux"
	}
	switch {
	case strings.Contains(ua, "iPad"):
		device = "tablet"
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "Android"):
		device = "mobile"
	default:
		device = "desktop"
	}
	return browser, osName, device
}

// buildReadStorage resolves the store that physically holds the blob (via
// blob_locations, mode-aware) and hydrates its Storage. Falls back to the
// primary store when no location row exists.
func buildReadStorage(ctx context.Context, tx bun.IDB, blobID uuid.UUID, fallbackPath string) (storage.Storage, error) {
	s, _, err := store.ResolveReadStore(ctx, tx, blobID, fallbackPath)
	if err != nil {
		return nil, errors.New("no active storage configured")
	}
	return store.BuildStorage(ctx, tx, s)
}
