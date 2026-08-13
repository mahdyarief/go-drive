package handler

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/security"
	"go-drive/server/internal/store"
	"go-drive/server/internal/tenant"
)

// ListUploadLinks returns all upload links in the workspace.
func ListUploadLinks(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var links []model.UploadLink
		if err := tx.NewSelect().Model(&links).Order("created_at DESC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing upload links: "+err.Error())
			return
		}
		Success(c, gin.H{"links": links})
	}
}

// CreateUploadLink creates an upload link. Body: { folderId?, name,
// maxFiles?, maxFileSize?, allowedMimeTypes?, password?, expiresAt? }.
func CreateUploadLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		orgSlug := c.GetString("org_slug")
		ctx := c.Request.Context()

		var req struct {
			FolderID         *uuid.UUID `json:"folderId"`
			Name             string     `json:"name"`
			MaxFiles         *int       `json:"maxFiles"`
			MaxFileSize      *int64     `json:"maxFileSize"`
			AllowedMimeTypes []string   `json:"allowedMimeTypes"`
			Password         string     `json:"password"`
			ExpiresAt        *time.Time `json:"expiresAt"`
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
		if req.FolderID != nil {
			if err := folderExists(ctx, tx, *req.FolderID); err != nil {
				Err(c, http.StatusNotFound, "folder not found")
				return
			}
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

		link := &model.UploadLink{
			ID:               uuid.New(),
			UserID:           userID,
			FolderID:         req.FolderID,
			Token:            token,
			Name:             req.Name,
			MaxFiles:         req.MaxFiles,
			MaxFileSize:      req.MaxFileSize,
			AllowedMimeTypes: req.AllowedMimeTypes,
			FilesUploaded:    0,
			HasPassword:      req.Password != "",
			PasswordHash:     hash,
			ExpiresAt:        req.ExpiresAt,
			IsActive:         true,
		}
		if _, err := tx.NewInsert().Model(link).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "creating upload link: "+err.Error())
			return
		}
		if err := registerLinkToken(ctx, db, token, orgSlug, "upload", link.ID.String()); err != nil {
			Err(c, http.StatusInternalServerError, "registering link token: "+err.Error())
			return
		}
		Created(c, gin.H{"link": link})
	}
}

// UpdateUploadLink updates an upload link. Body: { name?, maxFiles?,
// maxFileSize?, allowedMimeTypes?, password?, expiresAt?, isActive? }.
func UpdateUploadLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid upload link id")
			return
		}
		var req struct {
			Name             *string    `json:"name"`
			MaxFiles         *int       `json:"maxFiles"`
			MaxFileSize      *int64     `json:"maxFileSize"`
			AllowedMimeTypes *[]string  `json:"allowedMimeTypes"`
			Password         *string    `json:"password"`
			ExpiresAt        *time.Time `json:"expiresAt"`
			IsActive         *bool      `json:"isActive"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}

		var link model.UploadLink
		if err := tx.NewSelect().Model(&link).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "upload link not found")
			return
		}
		u := tx.NewUpdate().Model((*model.UploadLink)(nil))
		if req.Name != nil {
			if n := trimSpace(*req.Name); n != "" {
				u.Set("name = ?", n)
			}
		}
		if req.MaxFiles != nil {
			u.Set("max_files = ?", *req.MaxFiles)
		}
		if req.MaxFileSize != nil {
			u.Set("max_file_size = ?", *req.MaxFileSize)
		}
		if req.AllowedMimeTypes != nil {
			u.Set("allowed_mime_types = ?", *req.AllowedMimeTypes)
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
		if req.IsActive != nil {
			u.Set("is_active = ?", *req.IsActive)
		}
		u.Set("updated_at = ?", time.Now())
		if _, err := u.Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating upload link: "+err.Error())
			return
		}
		var updated model.UploadLink
		if err := tx.NewSelect().Model(&updated).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "reloading upload link: "+err.Error())
			return
		}
		Success(c, gin.H{"link": updated})
	}
}

// DeleteUploadLink removes an upload link and its registry token.
func DeleteUploadLink(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid upload link id")
			return
		}
		var link model.UploadLink
		if err := tx.NewSelect().Model(&link).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "upload link not found")
			return
		}
		if _, err := tx.NewDelete().Model((*model.UploadLink)(nil)).Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "deleting upload link: "+err.Error())
			return
		}
		_ = deleteLinkToken(ctx, db, link.Token)
		Msg(c, "upload link deleted")
	}
}

// PublicUpload serves POST /api/upload/public — a multipart upload via an
// upload link token. Fields: token, password?, file. Enforces the link's
// maxFiles/maxFileSize/allowedMimeTypes/expiry and increments files_uploaded.
func PublicUpload(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		token := c.PostForm("token")
		if token == "" {
			token = c.Query("token")
		}
		if token == "" {
			Err(c, http.StatusBadRequest, "token is required")
			return
		}
		row, err := resolveLinkToken(ctx, db, token)
		if err != nil {
			Err(c, http.StatusInternalServerError, "resolving token: "+err.Error())
			return
		}
		if row == nil || row.LinkType != "upload" {
			Err(c, http.StatusNotFound, "link not found")
			return
		}
		linkID, err := uuid.Parse(row.LinkID)
		if err != nil {
			Err(c, http.StatusNotFound, "link not found")
			return
		}
		tx, err := tenant.OpenTx(ctx, db, row.OrgSlug)
		if err != nil {
			Err(c, http.StatusInternalServerError, "opening tenant: "+err.Error())
			return
		}
		defer finishPublicTx(c, tx)

		var link model.UploadLink
		if err := tx.NewSelect().Model(&link).Where("id = ?", linkID).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "link not found")
			return
		}
		if !link.IsActive {
			Err(c, http.StatusForbidden, "link is inactive")
			return
		}
		if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
			Err(c, http.StatusForbidden, "link has expired")
			return
		}
		if link.HasPassword && !security.VerifyLinkPassword(link.PasswordHash, linkPassword(c)) {
			Err(c, http.StatusUnauthorized, "invalid password")
			return
		}
		if link.MaxFiles != nil && link.FilesUploaded >= *link.MaxFiles {
			Err(c, http.StatusForbidden, "upload limit reached")
			return
		}

		fileHeader, err := c.FormFile("file")
		if err != nil {
			Err(c, http.StatusBadRequest, "file field is required")
			return
		}
		if fileHeader.Size > store.MaxFileSize {
			Err(c, http.StatusRequestEntityTooLarge, "file exceeds 100 MB limit")
			return
		}
		if link.MaxFileSize != nil && fileHeader.Size > *link.MaxFileSize {
			Err(c, http.StatusRequestEntityTooLarge, "file exceeds the link's size limit")
			return
		}
		contentType := fileHeader.Header.Get("Content-Type")
		if len(link.AllowedMimeTypes) > 0 {
			allowed := false
			for _, m := range link.AllowedMimeTypes {
				if strings.EqualFold(m, contentType) {
					allowed = true
					break
				}
			}
			if !allowed {
				Err(c, http.StatusUnsupportedMediaType, "file type is not allowed for this link")
				return
			}
		}

		// Resolve the write store (primary in replicate mode, quota-aware in
		// cumulative mode) and build the object key (display path).
		s, err := store.ResolveUploadStore(ctx, tx, fileHeader.Size)
		if err != nil {
			Err(c, http.StatusInternalServerError, "no active storage configured: "+err.Error())
			return
		}
		name := filepath.Base(fileHeader.Filename)
		objectKey := name
		if link.FolderID != nil {
			if dir, err := store.FolderPath(ctx, tx, *link.FolderID); err == nil && dir != "" {
				objectKey = dir + "/" + name
			}
		}
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(name))
		}

		blob, f, err := store.CreatePendingFileUpload(ctx, tx, link.UserID, link.FolderID, name, objectKey, contentType, fileHeader.Size)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		src, err := fileHeader.Open()
		if err != nil {
			Err(c, http.StatusInternalServerError, "opening upload: "+err.Error())
			return
		}
		defer src.Close()
		st, err := store.BuildStorage(ctx, tx, s)
		if err != nil {
			Err(c, http.StatusInternalServerError, "building storage: "+err.Error())
			return
		}
		if err := st.Upload(ctx, objectKey, src, contentType); err != nil {
			Err(c, http.StatusInternalServerError, "uploading to storage: "+err.Error())
			return
		}
		if err := store.MarkFileUploadReady(ctx, tx, f.ID, blob.ID, s.ID, objectKey); err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := tx.NewUpdate().Model((*model.UploadLink)(nil)).
			Set("files_uploaded = files_uploaded + 1, updated_at = ?", time.Now()).
			Where("id = ?", link.ID).
			Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating upload count: "+err.Error())
			return
		}
		// Replication fanout to writable replicas (M7); non-fatal. Only in
		// replicate mode — cumulative mode keeps each file on a single store.
		if mode, err := store.GetStorageMode(ctx, tx); err == nil && mode == "replicate" {
			_ = store.SyncFileToStores(ctx, tx, f.ID, nil, nil, link.UserID)
		}
		Created(c, gin.H{"file": f})
	}
}
