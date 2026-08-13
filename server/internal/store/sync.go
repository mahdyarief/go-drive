package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/storage"
)

// SyncFileToStores replicates a single file's blob to every active writable
// store that does not already hold a copy. The blob is downloaded from the
// source store (or the primary when sourceStoreID is nil) and uploaded to each
// target at its display path. Each copy is recorded as a blob_location with
// origin 'replicated', and a replication_run_item is upserted when runID is
// non-nil. Runs serially against the caller's tx (bun.Tx is not concurrency
// safe, so SyncWorkspace, the only parallel caller, drives it serially).
func SyncFileToStores(ctx context.Context, tx bun.IDB, fileID uuid.UUID, sourceStoreID *uuid.UUID, runID *uuid.UUID, triggeredBy string) error {
	var f model.File
	if err := tx.NewSelect().Model(&f).Where("id = ?", fileID).Scan(ctx); err != nil {
		return fmt.Errorf("store: loading file: %w", err)
	}
	if f.Status != "ready" {
		return nil // nothing to replicate for non-ready files
	}
	var blob model.FileBlob
	if err := tx.NewSelect().Model(&blob).Where("id = ?", f.BlobID).Scan(ctx); err != nil {
		return fmt.Errorf("store: loading blob: %w", err)
	}

	// Resolve the source store + its storage path for this blob.
	var srcStoreID uuid.UUID
	srcPath := f.StoragePath
	if sourceStoreID != nil {
		var loc model.BlobLocation
		if err := tx.NewSelect().Model(&loc).
			Where("blob_id = ? AND store_id = ?", blob.ID, *sourceStoreID).
			Scan(ctx); err != nil {
			return fmt.Errorf("store: loading source location: %w", err)
		}
		srcStoreID = *sourceStoreID
		srcPath = loc.StoragePath
	} else {
		primary, err := ResolvePrimaryStore(ctx, tx)
		if err != nil {
			return fmt.Errorf("store: resolving primary source: %w", err)
		}
		srcStoreID = primary.ID
	}
	srcStore, err := loadStore(ctx, tx, srcStoreID)
	if err != nil {
		return err
	}
	srcSt, err := BuildStorage(ctx, tx, srcStore)
	if err != nil {
		return fmt.Errorf("store: building source storage: %w", err)
	}

	var stores []model.Store
	if err := tx.NewSelect().Model(&stores).
		Where("status = 'active' AND write_mode = 'write' AND id != ?", srcStoreID).
		Order("read_priority ASC").
		Scan(ctx); err != nil {
		return fmt.Errorf("store: listing target stores: %w", err)
	}

	now := time.Now()
	var lastErr error
	for i := range stores {
		target := &stores[i]
		n, err := tx.NewSelect().Model((*model.BlobLocation)(nil)).
			Where("blob_id = ? AND store_id = ?", blob.ID, target.ID).
			Count(ctx)
		if err != nil {
			lastErr = fmt.Errorf("store: checking location on %s: %w", target.ID, err)
			continue
		}
		if n > 0 {
			continue // already replicated
		}

		tgtSt, err := BuildStorage(ctx, tx, target)
		if err != nil {
			lastErr = fmt.Errorf("store: building target storage %s: %w", target.ID, err)
			if runID != nil {
				_ = upsertRunItem(ctx, tx, *runID, blob.ID, &srcStoreID, target.ID, "failed", err.Error(), &now)
			}
			continue
		}
		if err := copyBlob(ctx, srcSt, tgtSt, srcPath, blob.ObjectKey, blob.MimeType); err != nil {
			lastErr = fmt.Errorf("store: replicating to %s: %w", target.ID, err)
			if runID != nil {
				_ = upsertRunItem(ctx, tx, *runID, blob.ID, &srcStoreID, target.ID, "failed", err.Error(), &now)
			}
			continue
		}

		loc := &model.BlobLocation{
			ID:          uuid.New(),
			BlobID:      blob.ID,
			StoreID:     target.ID,
			StoragePath: blob.ObjectKey,
			State:       "available",
			Origin:      "replicated",
		}
		if _, err := tx.NewInsert().Model(loc).
			On("CONFLICT (blob_id, store_id) DO NOTHING").
			Exec(ctx); err != nil {
			lastErr = fmt.Errorf("store: recording replicated location: %w", err)
			continue
		}
		if runID != nil {
			_ = upsertRunItem(ctx, tx, *runID, blob.ID, &srcStoreID, target.ID, "completed", "", &now)
		}
	}
	return lastErr
}

// SyncWorkspace replicates every ready file in the workspace to all active
// writable stores ≠ source. It creates a replication_run (kind 'sync') and
// tracks progress. The loop is serial: bun.Tx is not safe for concurrent use,
// so a goroutine pool would corrupt the shared transaction.
func SyncWorkspace(ctx context.Context, tx bun.IDB, sourceStoreID *uuid.UUID, triggeredBy string) (*model.ReplicationRun, error) {
	var files []model.File
	if err := tx.NewSelect().Model(&files).Where("status = 'ready'").Scan(ctx); err != nil {
		return nil, fmt.Errorf("store: listing ready files: %w", err)
	}

	run := &model.ReplicationRun{
		ID:                uuid.New(),
		Kind:              "sync",
		Status:            "running",
		SourceStoreID:     sourceStoreID,
		TriggeredByUserID: triggeredBy,
		TotalItems:        len(files),
		StartedAt:         nowPtr(),
	}
	if _, err := tx.NewInsert().Model(run).Exec(ctx); err != nil {
		return nil, fmt.Errorf("store: creating replication run: %w", err)
	}

	runID := run.ID
	var failed int
	for _, f := range files {
		if err := SyncFileToStores(ctx, tx, f.ID, sourceStoreID, &runID, triggeredBy); err != nil {
			failed++
		}
		run.ProcessedItems++
		if _, err := tx.NewUpdate().Model((*model.ReplicationRun)(nil)).
			Set("processed_items = ?, updated_at = ?", run.ProcessedItems, time.Now()).
			Where("id = ?", run.ID).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("store: updating run progress: %w", err)
		}
	}

	run.FailedItems = failed
	run.Status = "completed"
	run.CompletedAt = nowPtr()
	if _, err := tx.NewUpdate().Model((*model.ReplicationRun)(nil)).
		Set("status = 'completed', failed_items = ?, completed_at = ?, updated_at = ?", failed, run.CompletedAt, time.Now()).
		Where("id = ?", run.ID).
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("store: completing replication run: %w", err)
	}
	return run, nil
}

// copyBlob streams an object from src to dst at the given paths.
func copyBlob(ctx context.Context, src, dst storage.Storage, srcPath, dstPath, mime string) error {
	r, _, err := src.Download(ctx, srcPath)
	if err != nil {
		return err
	}
	defer r.Close()
	if mime == "" {
		mime = "application/octet-stream"
	}
	return dst.Upload(ctx, dstPath, r, mime)
}

// upsertRunItem records a per-blob replication_run_item row.
func upsertRunItem(ctx context.Context, tx bun.IDB, runID uuid.UUID, blobID uuid.UUID, sourceStoreID *uuid.UUID, targetStoreID uuid.UUID, status, errMsg string, at *time.Time) error {
	item := &model.ReplicationRunItem{
		ID:            uuid.New(),
		RunID:         runID,
		BlobID:        blobID,
		SourceStoreID: sourceStoreID,
		TargetStoreID: targetStoreID,
		Status:        status,
		ErrorMessage:  errMsg,
		StartedAt:     at,
		CompletedAt:   at,
	}
	_, err := tx.NewInsert().Model(item).
		On("CONFLICT (run_id, blob_id, target_store_id) DO UPDATE SET status = EXCLUDED.status, attempt_count = replication_run_items.attempt_count + 1, error_message = EXCLUDED.error_message, completed_at = EXCLUDED.completed_at").
		Exec(ctx)
	return err
}

func loadStore(ctx context.Context, tx bun.IDB, id uuid.UUID) (*model.Store, error) {
	var s model.Store
	if err := tx.NewSelect().Model(&s).Where("id = ?", id).Scan(ctx); err != nil {
		return nil, fmt.Errorf("store: loading store %s: %w", id, err)
	}
	return &s, nil
}

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}
