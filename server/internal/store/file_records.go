// Package store contains the file-record and store-hydration logic for the
// file storage layer (Locker's server/stores equivalent).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
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

// quotaRefreshStaleAfter is how old a store's last_synced_at must be before
// ResolveUploadStoreReserved re-measures its quota from the provider.
const quotaRefreshStaleAfter = 5 * time.Minute

// ResolveUploadStore returns the store a new upload should target given the
// workspace's storage mode: the primary store in replicate mode, or the
// policy-aware quota-aware write store in cumulative mode.
func ResolveUploadStore(ctx context.Context, tx bun.IDB, size int64) (*model.Store, error) {
	return ResolveUploadStoreReserved(ctx, tx, size, nil)
}

// routingCandidate pairs a store with its estimated available bytes.
type routingCandidate struct {
	store *model.Store
	avail int64
}

// ResolveUploadStoreReserved is ResolveUploadStore with batch-aware
// reservations: reserved[storeID] bytes are subtracted from each store's
// available space so a single multi-file batch cannot overload one store.
func ResolveUploadStoreReserved(ctx context.Context, tx bun.IDB, size int64, reserved map[uuid.UUID]int64) (*model.Store, error) {
	mode, err := GetStorageMode(ctx, tx)
	if err != nil {
		return nil, err
	}
	if mode == "replicate" {
		return ResolvePrimaryStore(ctx, tx)
	}

	// Load the routing policy; absent rows behave as the default
	// most_available policy (i.e. the original ResolveWriteStore behavior).
	var policy model.StoreRoutingPolicy
	policyMode := "most_available"
	var priorityIDs []uuid.UUID
	cursor := 0
	hasPolicy := false
	if err := tx.NewSelect().Model(&policy).Limit(1).Scan(ctx); err == nil {
		hasPolicy = true
		if policy.Mode != "" {
			policyMode = policy.Mode
		}
		priorityIDs = policy.PriorityStoreIDs
		cursor = policy.RoundRobinCursor
	}

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

	// Best-effort refresh of stale quotas (last_synced_at older than 5m).
	refreshStaleQuotas(ctx, tx, stores)

	cands := make([]routingCandidate, 0, len(stores))
	for i := range stores {
		s := &stores[i]
		limit := s.QuotaLimit
		if s.Provider == "gdrive" {
			limit = s.ProviderQuotaLimit
		}
		avail := int64(math.MaxInt64) - s.QuotaUsed
		if limit > 0 {
			avail = limit - s.QuotaUsed
		}
		if r := reserved[s.ID]; r > 0 {
			avail -= r
		}
		cands = append(cands, routingCandidate{store: s, avail: avail})
	}

	// Stores with enough space for this upload.
	eligible := make([]routingCandidate, 0, len(cands))
	for _, c := range cands {
		if c.avail >= size {
			eligible = append(eligible, c)
		}
	}

	switch policyMode {
	case "round_robin":
		// When every store is below size, fall back to the one with the most
		// available space rather than failing.
		if len(eligible) == 0 {
			return mostAvailable(cands).store, nil
		}
		idx := cursor % len(eligible)
		if idx < 0 {
			idx = 0
		}
		sel := eligible[idx]
		// Persist the advanced cursor (best-effort; only when a policy row
		// already exists — reads never create one).
		if hasPolicy {
			_, _ = tx.NewUpdate().Model((*model.StoreRoutingPolicy)(nil)).
				Set("round_robin_cursor = ?, updated_at = ?", cursor+1, time.Now()).
				Where("workspace_id = ?", uuid.Nil).
				Exec(ctx)
		}
		return sel.store, nil
	case "priority":
		if len(eligible) > 0 {
			// First store whose id appears in priority_store_ids (order
			// matters), else fall back to the most available eligible store.
			for _, pid := range priorityIDs {
				for _, c := range eligible {
					if c.store.ID == pid {
						return c.store, nil
					}
				}
			}
			return mostAvailable(eligible).store, nil
		}
		return mostAvailable(cands).store, nil
	default: // most_available
		return mostAvailable(cands).store, nil
	}
}

// mostAvailable returns the candidate with the most available bytes.
func mostAvailable(cands []routingCandidate) routingCandidate {
	best := cands[0]
	for _, c := range cands[1:] {
		if c.avail > best.avail {
			best = c
		}
	}
	return best
}

// refreshStaleQuotas re-measures quotas for stores whose last_synced_at is
// older than quotaRefreshStaleAfter. Failures are non-fatal: cached values are
// kept and last_synced_at only advances on success. Only providers that report
// a real capacity (gdrive) are refreshed — local walks the whole store and S3
// reports nothing, so both are skipped.
func refreshStaleQuotas(ctx context.Context, tx bun.IDB, stores []model.Store) {
	cutoff := time.Now().Add(-quotaRefreshStaleAfter)
	for i := range stores {
		s := &stores[i]
		if s.Provider != "gdrive" {
			continue
		}
		if s.LastSyncedAt != nil && s.LastSyncedAt.After(cutoff) {
			continue
		}
		st, err := BuildStorage(ctx, tx, s)
		if err != nil {
			continue
		}
		used, limit, err := st.Quota(ctx)
		if err != nil || limit == 0 {
			continue
		}
		now := time.Now()
		if _, err := tx.NewUpdate().Model((*model.Store)(nil)).
			Where("id = ?", s.ID).
			Set("quota_used = ?, provider_quota_limit = ?", used, limit).
			Set("provider_quota_measured_at = ?, last_synced_at = ?, updated_at = ?", now, now, now).
			Exec(ctx); err != nil {
			continue
		}
		s.QuotaUsed = used
		s.ProviderQuotaLimit = limit
		s.ProviderQuotaAt = &now
		s.LastSyncedAt = &now
	}
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
		Set("status = 'ready', storage_path = ?, updated_at = ?", storagePath, now).
		Where("id = ?", fileID).
		Exec(ctx); err != nil {
		return fmt.Errorf("store: marking file ready: %w", err)
	}
	if _, err := tx.NewUpdate().Model((*model.FileBlob)(nil)).
		Set("state = 'ready', updated_at = ?", now).
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
	// blob_locations must be removed explicitly — SQLite has no FK cascade,
	// so orphaned rows would block same-name re-uploads (UNIQUE blob_id+store_id).
	if _, err := tx.NewDelete().Model((*model.BlobLocation)(nil)).Where("blob_id = ?", f.BlobID).Exec(ctx); err != nil {
		return fmt.Errorf("store: deleting blob locations: %w", err)
	}
	if _, err := tx.NewDelete().Model((*model.FileBlob)(nil)).Where("id = ?", f.BlobID).Exec(ctx); err != nil {
		return fmt.Errorf("store: deleting blob: %w", err)
	}
	return nil
}
