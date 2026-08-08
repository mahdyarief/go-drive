package handler

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/store"
)

// UploadFile handles POST /api/upload — multipart upload to the tenant's
// primary store. Fields: file, folderId?, fileId? (replace).
func UploadFile(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		fileHeader, err := c.FormFile("file")
		if err != nil {
			Err(c, http.StatusBadRequest, "file field is required")
			return
		}
		if fileHeader.Size > store.MaxFileSize {
			Err(c, http.StatusRequestEntityTooLarge, "file exceeds 100 MB limit")
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

		// Resolve the primary store for this workspace.
		s, err := store.ResolvePrimaryStore(ctx, tx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "no active storage configured: "+err.Error())
			return
		}

		// Build the object key (display path, e.g. docs/reports/q1.pdf).
		name := filepath.Base(fileHeader.Filename)
		objectKey := name
		if folderID != nil {
			if dir, err := store.FolderPath(ctx, tx, *folderID); err == nil && dir != "" {
				objectKey = dir + "/" + name
			}
		}

		// Create the pending file record.
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" {
			contentType = mime.TypeByExtension(filepath.Ext(name))
		}
		blob, f, err := store.CreatePendingFileUpload(ctx, tx, userID, folderID, name, objectKey, contentType, fileHeader.Size)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}

		// Stream the body to the primary store.
		src, err := fileHeader.Open()
		if err != nil {
			Err(c, http.StatusInternalServerError, "opening uploaded file: "+err.Error())
			return
		}
		defer src.Close()

		st, err := store.BuildStorage(ctx, tx, s)
		if err != nil {
			Err(c, http.StatusInternalServerError, "building storage: "+err.Error())
			return
		}
		if err := st.Upload(ctx, blob.ObjectKey, src, contentType); err != nil {
			Err(c, http.StatusInternalServerError, "upload failed: "+err.Error())
			return
		}

		// Mark ready + record primary blob location.
		if err := store.MarkFileUploadReady(ctx, tx, f.ID, blob.ID, s.ID, blob.ObjectKey); err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}

		// Replication fanout to writable replicas (M7). Runs synchronously in
		// the tenant tx; failures are non-fatal — the object is already on the
		// primary store, so the upload still succeeds.
		_ = store.SyncFileToStores(ctx, tx, f.ID, nil, nil, userID)

		Created(c, gin.H{"file": f})
	}
}
