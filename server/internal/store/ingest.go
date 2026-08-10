package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// IngestFromStore pulls objects from a read-only ingest store into the
// workspace. It lists objects under the store's root prefix, skips objects
// already tracked (existing blob_location on the store), tombstoned, or
// matching the .locker-store-test- probe marker, copies each into the primary
// store, records a blob_location with origin 'ingested', and fans out to other
// writable stores. Returns the number of objects ingested.
func IngestFromStore(ctx context.Context, tx bun.IDB, storeID uuid.UUID, triggeredBy string) (int, error) {
	s, err := loadStore(ctx, tx, storeID)
	if err != nil {
		return 0, err
	}
	st, err := BuildStorage(ctx, tx, s)
	if err != nil {
		return 0, fmt.Errorf("store: building ingest storage: %w", err)
	}
	prefix := StoreRootPrefix(s)
	objs, err := st.List(ctx, prefix)
	if err != nil {
		return 0, fmt.Errorf("store: listing ingest objects: %w", err)
	}

	primary, err := ResolvePrimaryStore(ctx, tx)
	if err != nil {
		return 0, fmt.Errorf("store: resolving primary: %w", err)
	}
	pst, err := BuildStorage(ctx, tx, primary)
	if err != nil {
		return 0, fmt.Errorf("store: building primary storage: %w", err)
	}
	// When the ingest store is also the primary store, its objects already
	// live in primary storage — copying them (download + re-upload) would
	// create duplicates (e.g. a second file in the same Google Drive
	// account). Only record the file as ready pointing at the existing path.
	sameAsPrimary := storeID == primary.ID

	// Existing root-level names for dedup.
	existing := map[string]bool{}
	var roots []model.File
	if err := tx.NewSelect().Model(&roots).Where("folder_id IS NULL AND status = 'ready'").Scan(ctx); err != nil {
		return 0, fmt.Errorf("store: listing root files: %w", err)
	}
	for _, f := range roots {
		existing[f.Name] = true
	}

	// The workspace storage mode decides how ingested objects are treated:
	// cumulative keeps the workspace copy on the primary store only, while
	// replicate fans it out to every other writable store.
	mode, _ := GetStorageMode(ctx, tx)

	ingested := 0
	for _, obj := range objs {
		if strings.Contains(obj.Path, ".locker-store-test-") {
			continue
		}
		display := DisplayPathFromKey(obj.Path, prefix)
		if display == "" {
			continue // not under this store's prefix (e.g. a bare prefix key)
		}
		// Skip tombstoned objects.
		n, err := tx.NewSelect().Model((*model.IngestTombstone)(nil)).
			Where("store_id = ? AND external_path = ?", storeID, obj.Path).
			Count(ctx)
		if err != nil {
			return ingested, fmt.Errorf("store: checking tombstone: %w", err)
		}
		if n > 0 {
			continue
		}
		// Skip objects already tracked on this store.
		n, err = tx.NewSelect().Model((*model.BlobLocation)(nil)).
			Where("store_id = ? AND storage_path = ?", storeID, obj.Path).
			Count(ctx)
		if err != nil {
			return ingested, fmt.Errorf("store: checking existing location: %w", err)
		}
		if n > 0 {
			continue
		}

		name := filepath.Base(display)
		if obj.Name != "" {
			// GDrive addresses objects by opaque file IDs (Path); use the real
			// file name so ingested files aren't named after their file ID.
			name = obj.Name
		}
		name = DedupName(name, existing)
		mime := "application/octet-stream" // storage objects don't carry content type
		blob, f, err := CreatePendingFileUpload(ctx, tx, triggeredBy, nil, name, display, mime, obj.Size)
		if err != nil {
			return ingested, err
		}
		// The object already lives in the primary store when the ingest
		// store IS the primary store (e.g. one GDrive store serving as both
		// workspace primary and ingest source). In that case skip the copy —
		// re-uploading would create a brand-new file in the same account.
		primaryPath := display
		if !sameAsPrimary {
			r, _, err := st.Download(ctx, obj.Path)
			if err != nil {
				return ingested, fmt.Errorf("store: downloading %s: %w", obj.Path, err)
			}
			if err := pst.Upload(ctx, display, r, mime); err != nil {
				r.Close()
				return ingested, fmt.Errorf("store: uploading to primary: %w", err)
			}
			r.Close()
		} else {
			primaryPath = obj.Path
		}

		// Mark file ready and record the primary copy (origin 'ingested').
		if err := markFileReady(ctx, tx, f.ID, blob.ID, primary.ID, primaryPath, "ingested"); err != nil {
			return ingested, err
		}
		// Record the ingest store's copy (origin 'ingested'). In replicate
		// mode the source object is a genuine replica, so it is reported as
		// available; in cumulative mode the row only exists so re-runs can
		// skip the object and is not an active workspace location.
		sourceState := "pending"
		if mode == "replicate" {
			sourceState = "available"
		}
		loc := &model.BlobLocation{
			ID:          uuid.New(),
			BlobID:      blob.ID,
			StoreID:     storeID,
			StoragePath: obj.Path,
			State:       sourceState,
			Origin:      "ingested",
		}
		if _, err := tx.NewInsert().Model(loc).
			On("CONFLICT (blob_id, store_id) DO NOTHING").
			Exec(ctx); err != nil {
			return ingested, fmt.Errorf("store: recording ingest location: %w", err)
		}
		// Fan out to other writable stores (replicate mode only).
		if mode == "replicate" {
			_ = SyncFileToStores(ctx, tx, f.ID, &storeID, nil, triggeredBy)
		}
		ingested++
	}
	return ingested, nil
}

// markFileReady flips a file/blob to ready and records a blob_location with
// the given origin for the primary store.
func markFileReady(ctx context.Context, tx bun.IDB, fileID, blobID, storeID uuid.UUID, storagePath, origin string) error {
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
		Origin:      origin,
	}
	if _, err := tx.NewInsert().Model(loc).
		On("CONFLICT (blob_id, store_id) DO NOTHING").
		Exec(ctx); err != nil {
		return fmt.Errorf("store: inserting blob_location: %w", err)
	}
	return nil
}
