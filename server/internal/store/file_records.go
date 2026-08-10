// Package store contains the file-record and store-hydration logic for the
// file storage layer (Locker's server/stores equivalent).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// MaxFileSize is the per-file upload limit (100 MB, matches Locker).
const MaxFileSize = 100 << 20 // 100 MB

// ResolvePrimaryStore returns the workspace's primary store. It prefers the
// explicit workspace_storage_settings row; if none exists it falls back to
// the first active write store ordered by read_priority.
func ResolvePrimaryStore(ctx context.Context, tx bun.IDB) (*model.Store, error) {
	var setting model.WorkspaceStorageSetting
	err := tx.NewSelect().Model(&setting).Limit(1).Scan(ctx)
	if err == nil {
		var s model.Store
		if err := tx.NewSelect().Model(&s).Where("id = ?", setting.PrimaryStoreID).Scan(ctx); err != nil {
			return nil, fmt.Errorf("store: loading primary: %w", err)
		}
		return &s, nil
	}

	var s model.Store
	if err := tx.NewSelect().Model(&s).
		Where("status = 'active' AND write_mode = 'write'").
		Order("read_priority ASC", "created_at ASC").
		Limit(1).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("store: no active write store: %w", err)
	}
	return &s, nil
}

// GetStorageMode returns the workspace's global storage mode ('replicate' or
// 'cumulative'). Absent settings rows default to cumulative.
func GetStorageMode(ctx context.Context, tx bun.IDB) (string, error) {
	var setting model.WorkspaceStorageSetting
	err := tx.NewSelect().Model(&setting).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "cumulative", nil
		}
		return "", fmt.Errorf("store: loading storage mode: %w", err)
	}
	if setting.StorageMode == "" {
		return "cumulative", nil
	}
	return setting.StorageMode, nil
}

// ResolveWriteStore returns the store a new file should be written to in
// cumulative mode: the explicit primary store when it has quota room,
// otherwise the first active writable store (by read_priority) whose quota
// fits size. quota_limit == 0 means the quota is unknown/uncapped and always
// fits. When every store looks full it falls back to the highest-priority
// store and lets the provider report the real error (quota_used is a
// best-effort estimate).
func ResolveWriteStore(ctx context.Context, tx bun.IDB, size int64) (*model.Store, error) {
	stores := make([]model.Store, 0, 8)
	if err := tx.NewSelect().Model(&stores).
		Where("status = 'active' AND write_mode = 'write'").
		Order("read_priority ASC", "created_at ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("store: listing write stores: %w", err)
	}
	if len(stores) == 0 {
		return nil, fmt.Errorf("store: no active write store")
	}

	// Prefer the explicit primary store when it can fit the upload.
	var primaryID *uuid.UUID
	var setting model.WorkspaceStorageSetting
	if err := tx.NewSelect().Model(&setting).Limit(1).Scan(ctx); err == nil {
		primaryID = &setting.PrimaryStoreID
	}
	if primaryID != nil {
		for i := range stores {
			if stores[i].ID == *primaryID && storeFits(&stores[i], size) {
				return &stores[i], nil
			}
		}
	}

	for i := range stores {
		if storeFits(&stores[i], size) {
			return &stores[i], nil
		}
	}
	return &stores[0], nil
}

// ResolveUploadStore returns the store a new upload should target given the
// workspace's storage mode: the primary store in replicate mode, or the
// quota-aware write store in cumulative mode.
func ResolveUploadStore(ctx context.Context, tx bun.IDB, size int64) (*model.Store, error) {
	mode, err := GetStorageMode(ctx, tx)
	if err != nil {
		return nil, err
	}
	if mode == "replicate" {
		return ResolvePrimaryStore(ctx, tx)
	}
	return ResolveWriteStore(ctx, tx, size)
}

// ResolveReadStore returns the store that physically holds the given blob and
// its storage path. In cumulative mode a file lives on a single (possibly
// non-primary) store, so reads resolve via blob_locations rather than the
// primary store. Falls back to the primary store + fallbackPath (replicate
// mode / legacy records where the location row may be missing).
func ResolveReadStore(ctx context.Context, tx bun.IDB, blobID uuid.UUID, fallbackPath string) (*model.Store, string, error) {
	var locs []model.BlobLocation
	if err := tx.NewSelect().Model(&locs).
		Where("blob_id = ? AND state = 'available'", blobID).
		Scan(ctx); err == nil && len(locs) > 0 {
		for _, loc := range locs {
			var s model.Store
			if err := tx.NewSelect().Model(&s).Where("id = ?", loc.StoreID).Scan(ctx); err != nil {
				continue
			}
			return &s, loc.StoragePath, nil
		}
	}
	s, err := ResolvePrimaryStore(ctx, tx)
	if err != nil {
		return nil, "", err
	}
	return s, fallbackPath, nil
}

// storeFits reports whether size fits within the store's quota.
// quota_limit == 0 is treated as unlimited (not configured).
func storeFits(s *model.Store, size int64) bool {
	if s.QuotaLimit <= 0 {
		return true
	}
	return s.QuotaUsed+size <= s.QuotaLimit
}

// CreatePendingFileUpload inserts a file_blobs + files row pair in the
// "uploading" state and returns both records. objectKey is the display path
// (e.g. docs/reports/q1.pdf) that becomes the blob's storage key.
func CreatePendingFileUpload(ctx context.Context, tx bun.IDB, userID string, folderID *uuid.UUID, name, objectKey, mime string, size int64) (*model.FileBlob, *model.File, error) {
	blob := &model.FileBlob{
		ID:          uuid.New(),
		CreatedByID: userID,
		ObjectKey:   objectKey,
		ByteSize:    size,
		MimeType:    mime,
		State:       "pending",
	}
	if _, err := tx.NewInsert().Model(blob).Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("store: inserting file_blob: %w", err)
	}

	f := &model.File{
		ID:              uuid.New(),
		UserID:          userID,
		FolderID:        folderID,
		BlobID:          blob.ID,
		Name:            name,
		MimeType:        mime,
		Size:            size,
		StoragePath:     objectKey,
		StorageProvider: "local", // overwritten after primary store is resolved
		Status:          "uploading",
	}
	if _, err := tx.NewInsert().Model(f).Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("store: inserting file: %w", err)
	}
	return blob, f, nil
}

// MarkFileUploadReady flips file + blob to ready and records the primary
// blob_location (origin primary_upload).
func MarkFileUploadReady(ctx context.Context, tx bun.IDB, fileID, blobID, storeID uuid.UUID, storagePath string) error {
	now := time.Now()

	if _, err := tx.NewUpdate().Model((*model.File)(nil)).
		Set("status = 'ready'", "storage_path = ?", storagePath, "updated_at = ?", now).
		Where("id = ?", fileID).
		Exec(ctx); err != nil {
		return fmt.Errorf("store: marking file ready: %w", err)
	}
	if _, err := tx.NewUpdate().Model((*model.FileBlob)(nil)).
		Set("state = 'ready'", "updated_at = ?", now).
		Where("id = ?", blobID).
		Exec(ctx); err != nil {
		return fmt.Errorf("store: marking blob ready: %w", err)
	}

	loc := &model.BlobLocation{
		ID:          uuid.New(),
		BlobID:      blobID,
		StoreID:     storeID,
		StoragePath: storagePath,
		State:       "available",
		Origin:      "primary_upload",
	}
	if _, err := tx.NewInsert().Model(loc).
		On("CONFLICT (blob_id, store_id) DO NOTHING").
		Exec(ctx); err != nil {
		return fmt.Errorf("store: inserting blob_location: %w", err)
	}
	return nil
}

// DeleteFileEverywhere removes the file record and all its blob locations.
func DeleteFileEverywhere(ctx context.Context, tx bun.IDB, fileID uuid.UUID) error {
	var f model.File
	if err := tx.NewSelect().Model(&f).Where("id = ?", fileID).Scan(ctx); err != nil {
		return fmt.Errorf("store: loading file: %w", err)
	}
	if _, err := tx.NewDelete().Model((*model.File)(nil)).Where("id = ?", fileID).Exec(ctx); err != nil {
		return fmt.Errorf("store: deleting file: %w", err)
	}
	// blob_locations + file_blobs cascade on blob delete
	if _, err := tx.NewDelete().Model((*model.FileBlob)(nil)).Where("id = ?", f.BlobID).Exec(ctx); err != nil {
		return fmt.Errorf("store: deleting blob: %w", err)
	}
	return nil
}
