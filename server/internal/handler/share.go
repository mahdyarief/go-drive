package handler

import (
	"context"
	"errors"
	"net/http"
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

// ListShareLinks returns all share links in the workspace.
func ListShareLinks(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var links []model.ShareLink
		if err := tx.NewSelect().Model(&links).Order("created_at DESC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing share links: "+err.Error())
			return
		}
		Success(c, gin.H{"links": links})
	}
}

// CreateShareLink creates a share link. Body: { fileId?, folderId?, access?,
// password?, expiresAt?, maxDownloads? }. Exactly one of fileId/folderId.
func CreateShareLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		orgSlug := c.GetString("org_slug")
		ctx := c.Request.Context()

		var req struct {
			FileID       *uuid.UUID `json:"fileId"`
			FolderID     *uuid.UUID `json:"folderId"`
			Access       string     `json:"access"`
			Password     string     `json:"password"`
			ExpiresAt    *time.Time `json:"expiresAt"`
			MaxDownloads *int       `json:"maxDownloads"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if (req.FileID == nil) == (req.FolderID == nil) {
			Err(c, http.StatusBadRequest, "exactly one of fileId or folderId is required")
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
			access = "download"
		}
		if access != "download" && access != "view" {
			Err(c, http.StatusBadRequest, "access must be download or view")
			return
		}
		hash, err := security.HashLinkPassword(req.Password)
		if err != nil {
			Err(c, http.StatusInternalServerError, "hashing password: "+err.Error())
			return
		}
		token, err := security.Token32()
		if err != nil {
			Err(c, http.StatusInternalServerError, "generating token: "+err.Error())
			return
		}

		link := &model.ShareLink{
			ID:            uuid.New(),
			UserID:        userID,
			FileID:        req.FileID,
			FolderID:      req.FolderID,
			Token:         token,
			Access:        access,
			HasPassword:   req.Password != "",
			PasswordHash:  hash,
			ExpiresAt:     req.ExpiresAt,
			MaxDownloads:  req.MaxDownloads,
			DownloadCount: 0,
			IsActive:      true,
		}
		if _, err := tx.NewInsert().Model(link).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "creating share link: "+err.Error())
			return
		}
		if err := registerLinkToken(ctx, db, token, orgSlug, "share", link.ID.String()); err != nil {
			Err(c, http.StatusInternalServerError, "registering link token: "+err.Error())
			return
		}
		Created(c, gin.H{"link": link})
	}
}

// UpdateShareLink updates a share link. Body: { access?, password?,
// expiresAt?, maxDownloads?, isActive? }.
func UpdateShareLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid share link id")
			return
		}
		var req struct {
			Access       *string    `json:"access"`
			Password     *string    `json:"password"`
			ExpiresAt    *time.Time `json:"expiresAt"`
			MaxDownloads *int       `json:"maxDownloads"`
			IsActive     *bool      `json:"isActive"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}

		var link model.ShareLink
		if err := tx.NewSelect().Model(&link).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "share link not found")
			return
		}
		u := tx.NewUpdate().Model((*model.ShareLink)(nil))
		if req.Access != nil && *req.Access != "" && *req.Access != link.Access {
			if *req.Access != "download" && *req.Access != "view" {
				Err(c, http.StatusBadRequest, "access must be download or view")
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
		if req.ExpiresAt != nil {
			u.Set("expires_at = ?", *req.ExpiresAt)
		}
		if req.MaxDownloads != nil {
			u.Set("max_downloads = ?", *req.MaxDownloads)
		}
		if req.IsActive != nil {
			u.Set("is_active = ?", *req.IsActive)
		}
		u.Set("updated_at = ?", time.Now())
		if _, err := u.Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating share link: "+err.Error())
			return
		}
		var updated model.ShareLink
		if err := tx.NewSelect().Model(&updated).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "reloading share link: "+err.Error())
			return
		}
		Success(c, gin.H{"link": updated})
	}
}

// DeleteShareLink removes a share link and its registry token.
func DeleteShareLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid share link id")
			return
		}
		var link model.ShareLink
		if err := tx.NewSelect().Model(&link).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "share link not found")
			return
		}
		if _, err := tx.NewDelete().Model((*model.ShareLink)(nil)).Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "deleting share link: "+err.Error())
			return
		}
		_ = deleteLinkToken(ctx, db, link.Token)
		Msg(c, "share link deleted")
	}
}

// PublicShareLink serves GET /api/shared/:token — file or folder browse.
func PublicShareLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tx, link, ok := openShareLink(c, db, "", false)
		if !ok {
			return
		}
		defer finishPublicTx(c, tx)

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

// PublicShareDownload serves GET /api/shared/:token/download?fileId= — returns
// a signed URL for a file within the link's scope and increments download_count.
func PublicShareDownload(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		pw := linkPassword(c)
		tx, link, ok := openShareLink(c, db, pw, true)
		if !ok {
			return
		}
		defer finishPublicTx(c, tx)

		f, ok := resolveSharedFile(c, tx, link.FileID, link.FolderID, c.Query("fileId"))
		if !ok {
			return
		}
		s, path, err := store.ResolveReadStore(ctx, tx, f.BlobID, f.StoragePath)
		if err != nil {
			Err(c, http.StatusInternalServerError, "no active storage configured")
			return
		}
		st, err := store.BuildStorage(ctx, tx, s)
		if err != nil {
			Err(c, http.StatusInternalServerError, "building storage: "+err.Error())
			return
		}
		url, err := st.GetSignedURL(ctx, path, time.Hour)
		if err != nil {
			if errors.Is(err, storage.ErrNotSupported) {
				Err(c, http.StatusNotImplemented, "signed URLs not supported for this provider")
				return
			}
			Err(c, http.StatusInternalServerError, "signing url: "+err.Error())
			return
		}
		if err := bumpShareCount(ctx, tx, link.ID); err != nil {
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

// PublicShareRaw serves GET /api/shared/:token/raw?fileId= — streams the file
// bytes directly and increments download_count.
func PublicShareRaw(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		pw := linkPassword(c)
		tx, link, ok := openShareLink(c, db, pw, true)
		if !ok {
			return
		}
		defer finishPublicTx(c, tx)

		f, ok := resolveSharedFile(c, tx, link.FileID, link.FolderID, c.Query("fileId"))
		if !ok {
			return
		}
		s, path, err := store.ResolveReadStore(ctx, tx, f.BlobID, f.StoragePath)
		if err != nil {
			Err(c, http.StatusInternalServerError, "no active storage configured")
			return
		}
		st, err := store.BuildStorage(ctx, tx, s)
		if err != nil {
			Err(c, http.StatusInternalServerError, "building storage: "+err.Error())
			return
		}
		r, size, err := st.Download(ctx, path)
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
		if err := bumpShareCount(ctx, tx, link.ID); err != nil {
			return // response already sent
		}
	}
}

// openShareLink resolves a share token, opens the tenant tx, loads and
// validates the link. whenDownload additionally enforces the 'view' access
// restriction and maxDownloads limit.
func openShareLink(c *gin.Context, db *bun.DB, password string, whenDownload bool) (bun.Tx, *model.ShareLink, bool) {
	ctx := c.Request.Context()
	row, err := resolveLinkToken(ctx, db, c.Param("token"))
	if err != nil {
		Err(c, http.StatusInternalServerError, "resolving token: "+err.Error())
		return bun.Tx{}, nil, false
	}
	if row == nil || row.LinkType != "share" {
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

	var link model.ShareLink
	if err := tx.NewSelect().Model(&link).Where("id = ?", linkID).Scan(ctx); err != nil {
		Err(c, http.StatusNotFound, "link not found")
		return bun.Tx{}, &link, false
	}
	if !link.IsActive {
		Err(c, http.StatusForbidden, "link is inactive")
		return bun.Tx{}, &link, false
	}
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		Err(c, http.StatusForbidden, "link has expired")
		return bun.Tx{}, &link, false
	}
	if link.MaxDownloads != nil && link.DownloadCount >= *link.MaxDownloads {
		Err(c, http.StatusForbidden, "download limit reached")
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
	ok = true
	return tx, &link, true
}

// resolveSharedFile loads a file that is within the link's scope. For a file
// link the file must be the link target; for a folder link it must be a
// descendant of the folder. fileID and folderID are the link's target pointers.
func resolveSharedFile(c *gin.Context, tx bun.Tx, fileID, folderID *uuid.UUID, fileIDParam string) (model.File, bool) {
	ctx := c.Request.Context()
	var target uuid.UUID
	if fileIDParam != "" {
		id, err := uuid.Parse(fileIDParam)
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid fileId")
			return model.File{}, false
		}
		target = id
	} else if fileID != nil {
		target = *fileID
	} else {
		Err(c, http.StatusBadRequest, "fileId is required")
		return model.File{}, false
	}

	var f model.File
	if err := tx.NewSelect().Model(&f).Where("id = ? AND status = 'ready'", target).Scan(ctx); err != nil {
		Err(c, http.StatusNotFound, "file not found")
		return model.File{}, false
	}
	if fileID != nil {
		if f.ID != *fileID {
			Err(c, http.StatusForbidden, "file is not part of this link")
			return model.File{}, false
		}
		return f, true
	}
	if f.FolderID == nil {
		Err(c, http.StatusForbidden, "file is not part of this link")
		return model.File{}, false
	}
	ids, err := store.FolderSubtree(ctx, tx, *folderID)
	if err != nil {
		Err(c, http.StatusInternalServerError, err.Error())
		return model.File{}, false
	}
	for _, id := range ids {
		if id == *f.FolderID {
			return f, true
		}
	}
	Err(c, http.StatusForbidden, "file is not part of this link")
	return model.File{}, false
}

// bumpShareCount increments a share link's download_count and last_accessed_at.
func bumpShareCount(ctx context.Context, tx bun.Tx, linkID uuid.UUID) error {
	_, err := tx.NewUpdate().Model((*model.ShareLink)(nil)).
		Set("download_count = download_count + 1, last_accessed_at = ?", time.Now()).
		Where("id = ?", linkID).
		Exec(ctx)
	return err
}

// finishPublicTx commits on success (2xx) and rolls back otherwise. Public
// handlers open their own tenant tx, so they must close it explicitly.
func finishPublicTx(c *gin.Context, tx bun.Tx) {
	if c.Writer.Status() >= 400 || c.IsAborted() {
		_ = tx.Rollback()
		return
	}
	_ = tx.Commit()
}

// linkPassword returns the link password from the X-Link-Password header or
// the ?password= query parameter.
func linkPassword(c *gin.Context) string {
	if p := c.GetHeader("X-Link-Password"); p != "" {
		return p
	}
	return c.Query("password")
}

// fileReadyExists reports whether a ready file exists by id.
func fileReadyExists(ctx context.Context, tx bun.IDB, id uuid.UUID) error {
	return tx.NewSelect().Model((*model.File)(nil)).Where("id = ? AND status = 'ready'", id).Scan(ctx, &model.File{})
}
