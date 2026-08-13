package handler

import (
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/store"
)

// PublicUploadByAPIKey serves POST /api/v1/uploads — a multipart batch upload
// authenticated by an API key (middleware.RequireAPIKey). The tenant
// transaction is bootstrapped by middleware.APIKeyTenantTx from the key's
// org_slug, so no session or X-Org-Slug header is needed. It replicates the
// exact store pipeline from upload.go's uploadOne inline (same shape:
// {files: [...], failed: [...]}).
func PublicUploadByAPIKey(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")

		form, err := c.MultipartForm()
		if err != nil {
			Err(c, http.StatusBadRequest, "expected multipart form data")
			return
		}

		headers := append(form.File["files"], form.File["file"]...)
		if len(headers) == 0 {
			Err(c, http.StatusBadRequest, "at least one file is required")
			return
		}

		var folderID *uuid.UUID
		if f := strings.TrimSpace(c.PostForm("folderId")); f != "" {
			id, err := uuid.Parse(f)
			if err != nil {
				Err(c, http.StatusBadRequest, "invalid folderId")
				return
			}
			folderID = &id
		}

		uploaded := make([]uploadedFile, 0, len(headers))
		failed := make([]failedUpload, 0)

		// Enforce the org's allocated storage quota (local providers only)
		// before writing anything — reject the whole batch with 413.
		var batchSize int64
		for _, h := range headers {
			batchSize += h.Size
		}
		if err := checkOrgUploadQuota(c.Request.Context(), db, tx, c.GetString("org_slug"), batchSize); err != nil {
			Err(c, http.StatusRequestEntityTooLarge, err.Error())
			return
		}

		for _, h := range headers {
			f, err := uploadOnePublic(c, tx, userID, folderID, h)
			if err != nil {
				failed = append(failed, failedUpload{Name: filepath.Base(h.Filename), Error: err.Error()})
				continue
			}
			uploaded = append(uploaded, *f)
		}

		Success(c, gin.H{"files": uploaded, "failed": failed})
	}
}

// uploadOnePublic replicates upload.go's uploadOne pipeline for the
// API-key-authenticated public endpoint. It runs against the tenant tx opened
// by APIKeyTenantTx.
func uploadOnePublic(c *gin.Context, tx bun.Tx, userID string, folderID *uuid.UUID, h *multipart.FileHeader) (*uploadedFile, error) {
	ctx := c.Request.Context()

	if h.Size > store.MaxFileSize {
		return nil, errors.New("file exceeds 100 MB limit")
	}

	// Resolve the write store for this workspace (primary in replicate mode,
	// quota-aware in cumulative mode).
	s, err := store.ResolveUploadStore(ctx, tx, h.Size)
	if err != nil {
		return nil, fmt.Errorf("no active storage configured: %w", err)
	}

	// Build the object key (display path, e.g. docs/reports/q1.pdf).
	name := filepath.Base(h.Filename)
	objectKey := name
	if folderID != nil {
		if dir, err := store.FolderPath(ctx, tx, *folderID); err == nil && dir != "" {
			objectKey = dir + "/" + name
		}
	}

	contentType := h.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(name))
	}

	blob, f, err := store.CreatePendingFileUpload(ctx, tx, userID, folderID, name, objectKey, contentType, h.Size)
	if err != nil {
		return nil, err
	}

	src, err := h.Open()
	if err != nil {
		return nil, fmt.Errorf("opening uploaded file: %w", err)
	}
	defer src.Close()

	st, err := store.BuildStorage(ctx, tx, s)
	if err != nil {
		return nil, fmt.Errorf("building storage: %w", err)
	}
	if err := st.Upload(ctx, blob.ObjectKey, src, contentType); err != nil {
		return nil, fmt.Errorf("uploading to storage: %w", err)
	}

	if err := store.MarkFileUploadReady(ctx, tx, f.ID, blob.ID, s.ID, blob.ObjectKey); err != nil {
		return nil, err
	}

	// Replication fanout to writable replicas (M7); non-fatal. Only in
	// replicate storage mode.
	if mode, err := store.GetStorageMode(ctx, tx); err == nil && mode == "replicate" {
		_ = store.SyncFileToStores(ctx, tx, f.ID, nil, nil, userID)
	}

	return &uploadedFile{Name: f.Name, ID: f.ID, Size: f.Size}, nil
}
