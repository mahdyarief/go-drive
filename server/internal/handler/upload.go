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

type uploadedFile struct {
	Name string    `json:"name"`
	ID   uuid.UUID `json:"id"`
	Size int64     `json:"size"`
}

type failedUpload struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// UploadFile handles POST /api/t/upload — multipart batch upload to the
// tenant's store. Fields: files (repeatable), file (legacy single), folderId?.
// Files are streamed sequentially; per-file failures are collected in "failed"
// without failing the whole request.
func UploadFile(db *bun.DB) gin.HandlerFunc {
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

		// Track bytes reserved per store across the batch so the routing
		// policy does not pile every file of one batch onto a single store.
		reserved := make(map[uuid.UUID]int64)

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
			f, err := uploadOne(c, tx, userID, folderID, h, reserved)
			if err != nil {
				failed = append(failed, failedUpload{Name: filepath.Base(h.Filename), Error: err.Error()})
				continue
			}
			uploaded = append(uploaded, *f)
		}

		Success(c, gin.H{"files": uploaded, "failed": failed})
	}
}

func uploadOne(c *gin.Context, tx bun.Tx, userID string, folderID *uuid.UUID, h *multipart.FileHeader, reserved map[uuid.UUID]int64) (*uploadedFile, error) {
	ctx := c.Request.Context()

	if h.Size > store.MaxFileSize {
		return nil, errors.New("file exceeds 100 MB limit")
	}

	// Resolve the write store for this workspace (primary in replicate mode,
	// policy-aware quota-aware in cumulative mode). Reserved bytes from
	// earlier files in the batch reduce the chosen store's available space.
	s, err := store.ResolveUploadStoreReserved(ctx, tx, h.Size, reserved)
	if err != nil {
		return nil, fmt.Errorf("no active storage configured: %w", err)
	}
	reserved[s.ID] += h.Size

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
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	if err := store.MarkFileUploadReady(ctx, tx, f.ID, blob.ID, s.ID, blob.ObjectKey); err != nil {
		return nil, err
	}

	// Replication fanout to writable replicas (M7). Only in replicate mode;
	// cumulative mode keeps each file on a single store. Runs synchronously
	// in the tenant tx; failures are non-fatal — the object is already on
	// the write store, so the upload still succeeds.
	if mode, err := store.GetStorageMode(ctx, tx); err == nil && mode == "replicate" {
		_ = store.SyncFileToStores(ctx, tx, f.ID, nil, nil, userID)
	}

	auditLog(ctx, tx, userID, "file_upload", "file", f.ID.String(), map[string]any{"name": f.Name, "size": f.Size})

	return &uploadedFile{Name: f.Name, ID: f.ID, Size: f.Size}, nil
}
