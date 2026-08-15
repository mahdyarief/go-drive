package handler

import (
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"

	"go-drive/server/internal/tenant"
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
		// Commit the middleware transaction early to release the SQLite lock.
		// We'll manage our own transactions for each file upload.
		tx := c.MustGet("tenant_tx").(bun.Tx)
		if err := tx.Commit(); err != nil {
			Err(c, http.StatusInternalServerError, "failed to commit initial transaction")
			return
		}
		c.Set("tx_released", true)

		userID := c.GetString("user_id")
		orgSlug := c.GetString("org_slug")

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

		// Get tenant DB for SQLite mode
		tenantDB := db
		if db.Dialect().Name() == dialect.SQLite {
			tdb, terr := tenant.DB(c.Request.Context(), orgSlug)
			if terr != nil {
				Err(c, http.StatusInternalServerError, "failed to get tenant database")
				return
			}
			tenantDB = tdb
		}

		// Enforce the org's allocated storage quota (local providers only)
		// before writing anything — reject the whole batch with 413.
		var batchSize int64
		for _, h := range headers {
			batchSize += h.Size
		}
		quotaTx, err := tenantDB.BeginTx(c.Request.Context(), nil)
		if err != nil {
			Err(c, http.StatusInternalServerError, "failed to begin quota transaction")
			return
		}
		if err := checkOrgUploadQuota(c.Request.Context(), tenantDB, quotaTx, orgSlug, batchSize); err != nil {
			quotaTx.Rollback()
			Err(c, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		quotaTx.Commit()

		for _, h := range headers {
			f, err := uploadOne(c, tenantDB, userID, folderID, h, reserved)
			if err != nil {
				failed = append(failed, failedUpload{Name: filepath.Base(h.Filename), Error: err.Error()})
				continue
			}
			uploaded = append(uploaded, *f)
		}

		Success(c, gin.H{"files": uploaded, "failed": failed})
	}
}

// uploadOne handles uploading a single file with 3-phase pattern to minimize
// database lock time in SQLite mode. Phase 1: setup (short tx), Phase 2: storage
// I/O (no tx), Phase 3: finalize (short tx). Includes retry logic for SQLITE_BUSY.
func uploadOne(c *gin.Context, db *bun.DB, userID string, folderID *uuid.UUID, h *multipart.FileHeader, reserved map[uuid.UUID]int64) (*uploadedFile, error) {
	if h.Size > store.MaxFileSize {
		return nil, errors.New("file exceeds 100 MB limit")
	}

	// Retry wrapper for SQLITE_BUSY errors (max 3 attempts with backoff)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
		}

		result, err := uploadOneAttempt(c, db, userID, folderID, h, reserved)
		if err == nil {
			return result, nil
		}

		// Check if this is a SQLITE_BUSY error
		if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
			lastErr = err
			continue
		}

		// Non-retryable error
		return nil, err
	}

	return nil, fmt.Errorf("upload failed after retries (SQLITE_BUSY): %w", lastErr)
}

// uploadOneAttempt performs a single upload attempt with 3-phase pattern.
func uploadOneAttempt(c *gin.Context, db *bun.DB, userID string, folderID *uuid.UUID, h *multipart.FileHeader, reserved map[uuid.UUID]int64) (*uploadedFile, error) {
	ctx := c.Request.Context()

	// === PHASE 1: Setup (short transaction) ===
	// Create a new transaction for setup operations. This is fast and minimizes lock time.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting setup transaction: %w", err)
	}
	defer tx.Rollback()

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

	// Create pending file records (fast DB insert)
	blob, f, err := store.CreatePendingFileUpload(ctx, tx, userID, folderID, name, objectKey, contentType, h.Size)
	if err != nil {
		return nil, err
	}

	// Build storage backend (reads config from DB, fast)
	st, err := store.BuildStorage(ctx, tx, s)
	if err != nil {
		return nil, fmt.Errorf("building storage: %w", err)
	}

	// Commit Phase 1 transaction to release the lock before storage I/O
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing setup transaction: %w", err)
	}

	// === PHASE 2: Storage I/O (outside transaction) ===
	// Open multipart file and upload to storage backend (S3/GDrive/local).
	// This is the slow part - can take seconds to minutes for large files.
	// By doing this outside the transaction, we don't hold the SQLite write lock.
	src, err := h.Open()
	if err != nil {
		return nil, fmt.Errorf("opening uploaded file: %w", err)
	}
	defer src.Close()

	if err := st.Upload(ctx, blob.ObjectKey, src, contentType); err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	// === PHASE 3: Finalization (new transaction) ===
	// Start a new transaction for marking the upload as ready.
	// This is fast (just DB updates) and minimizes lock time.
	finalizeTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("starting finalize transaction: %w", err)
	}
	defer finalizeTx.Rollback()

	if err := store.MarkFileUploadReady(ctx, finalizeTx, f.ID, blob.ID, s.ID, blob.ObjectKey); err != nil {
		return nil, err
	}

	// Replication fanout to writable replicas (M7). Only in replicate mode;
	// cumulative mode keeps each file on a single store. Runs synchronously
	// in the tenant tx; failures are non-fatal — the object is already on
	// the write store, so the upload still succeeds.
	if mode, err := store.GetStorageMode(ctx, finalizeTx); err == nil && mode == "replicate" {
		_ = store.SyncFileToStores(ctx, finalizeTx, f.ID, nil, nil, userID)
	}

	auditLog(ctx, finalizeTx, userID, "file_upload", "file", f.ID.String(), map[string]any{"name": f.Name, "size": f.Size})

	if err := finalizeTx.Commit(); err != nil {
		return nil, fmt.Errorf("committing finalize transaction: %w", err)
	}

	// Dispatch webhook event (fire-and-forget, uses its own tx)
	go store.DispatchEvent(ctx, finalizeTx, db, c.GetString("org_slug"), "file.upload", map[string]any{
		"file_id": f.ID.String(),
		"name":    f.Name,
		"size":    f.Size,
	})

	return &uploadedFile{Name: f.Name, ID: f.ID, Size: f.Size}, nil
}
