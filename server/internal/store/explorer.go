// Package store contains the file-record and store-hydration logic for the
// file storage layer (Locker's server/stores equivalent).
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// FolderSubtree returns folderID plus every descendant folder ID (BFS walk).
// Used by folder delete (recursive) and folder-move cycle detection.
func FolderSubtree(ctx context.Context, tx bun.IDB, folderID uuid.UUID) ([]uuid.UUID, error) {
	ids := []uuid.UUID{folderID}
	queue := []uuid.UUID{folderID}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		var children []uuid.UUID
		if err := tx.NewSelect().Model((*model.Folder)(nil)).
			Where("parent_id = ?", parent).
			Column("id").
			Scan(ctx, &children); err != nil {
			return nil, fmt.Errorf("store: listing children of %s: %w", parent, err)
		}
		for _, id := range children {
			ids = append(ids, id)
			queue = append(queue, id)
		}
		if len(ids) > 10_000 {
			return nil, fmt.Errorf("store: folder subtree exceeds 10000 nodes")
		}
	}
	return ids, nil
}

// DeleteFolderRecursive moves every file under folderID (and descendants) to
// the root and deletes the folders. Returns the number of folders deleted.
func DeleteFolderRecursive(ctx context.Context, tx bun.IDB, folderID uuid.UUID) (int, error) {
	ids, err := FolderSubtree(ctx, tx, folderID)
	if err != nil {
		return 0, err
	}
	if _, err := tx.NewUpdate().Model((*model.File)(nil)).
		Set("folder_id = NULL, updated_at = ?", time.Now()).
		Where("folder_id IN (?)", bun.In(ids)).
		Exec(ctx); err != nil {
		return 0, fmt.Errorf("store: moving files to root: %w", err)
	}
	res, err := tx.NewDelete().Model((*model.Folder)(nil)).
		Where("id IN (?)", bun.In(ids)).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: deleting folders: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// DeleteFolderRecursiveWithFiles deletes all files (from storage + DB) under
// folderID and its descendants, then deletes the folders. Returns the number
// of folders deleted and files deleted.
func DeleteFolderRecursiveWithFiles(ctx context.Context, tx bun.IDB, folderID uuid.UUID) (int, int, error) {
	ids, err := FolderSubtree(ctx, tx, folderID)
	if err != nil {
		return 0, 0, err
	}

	// Get all files in these folders
	var files []model.File
	if err := tx.NewSelect().Model(&files).
		Where("folder_id IN (?)", bun.In(ids)).
		Scan(ctx); err != nil {
		return 0, 0, fmt.Errorf("store: listing files: %w", err)
	}

	// Delete each file from storage backend and DB
	filesDeleted := 0
	for _, f := range files {
		// Delete from storage backend
		locs, err := BlobLocationsForFile(ctx, tx, f.BlobID)
		if err != nil {
			return 0, 0, fmt.Errorf("store: loading blob locations: %w", err)
		}
		for _, loc := range locs {
			var s model.Store
			if err := tx.NewSelect().Model(&s).Where("id = ?", loc.StoreID).Scan(ctx); err != nil {
				continue
			}
			if st, err := BuildStorage(ctx, tx, &s); err == nil {
				_ = st.Delete(ctx, loc.StoragePath)
			}
		}

		// Delete from DB
		if err := DeleteFileEverywhere(ctx, tx, f.ID); err != nil {
			return 0, 0, fmt.Errorf("store: deleting file %s: %w", f.ID, err)
		}
		filesDeleted++
	}

	// Delete folders
	res, err := tx.NewDelete().Model((*model.Folder)(nil)).
		Where("id IN (?)", bun.In(ids)).
		Exec(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("store: deleting folders: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), filesDeleted, nil
}

// RenameFile updates a file's name and folder and keeps its blob object_key
// and every blob_location storage_path in sync with the new display path.
// objectKey must be the new display path (folderPath/name); pass "" to leave
// storage keys untouched (e.g. folder-only changes that keep the path).
// Returns the updated file record.
func RenameFile(ctx context.Context, tx bun.IDB, fileID uuid.UUID, name string, folderID *uuid.UUID, objectKey string) (*model.File, error) {
	var f model.File
	if err := tx.NewSelect().Model(&f).Where("id = ?", fileID).Scan(ctx); err != nil {
		return nil, fmt.Errorf("store: loading file: %w", err)
	}

	u := tx.NewUpdate().Model((*model.File)(nil))
	if name != "" && name != f.Name {
		u.Set("name = ?", name)
		f.Name = name
	}
	if folderID != nil {
		u.Set("folder_id = ?", *folderID)
		f.FolderID = folderID
	}
	if objectKey != "" {
		u.Set("storage_path = ?", objectKey)
		f.StoragePath = objectKey
	}
	u.Set("updated_at = ?", time.Now())
	if _, err := u.Where("id = ?", fileID).Exec(ctx); err != nil {
		return nil, fmt.Errorf("store: updating file: %w", err)
	}

	if objectKey != "" {
		if _, err := tx.NewUpdate().Model((*model.FileBlob)(nil)).
			Set("object_key = ?, updated_at = ?", objectKey, time.Now()).
			Where("id = ?", f.BlobID).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("store: updating blob object_key: %w", err)
		}
		if _, err := tx.NewUpdate().Model((*model.BlobLocation)(nil)).
			Set("storage_path = ?, updated_at = ?", objectKey, time.Now()).
			Where("blob_id = ?", f.BlobID).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("store: updating blob_location storage_path: %w", err)
		}
	}
	return &f, nil
}

// BlobLocationsForFile returns every blob_location row for a file's blob.
func BlobLocationsForFile(ctx context.Context, tx bun.IDB, blobID uuid.UUID) ([]model.BlobLocation, error) {
	var locs []model.BlobLocation
	if err := tx.NewSelect().Model(&locs).
		Where("blob_id = ?", blobID).
		Order("created_at ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("store: loading blob locations: %w", err)
	}
	return locs, nil
}
