package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/storage"
	"go-drive/server/internal/store"
)

// FileDownloadURL returns a time-limited download URL for a file (3600s TTL).
// Falls back to server-side streaming when the provider has no signed URLs.
func FileDownloadURL(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var f model.File
		if err := tx.NewSelect().Model(&f).Where("id = ?", c.Param("id")).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "file not found")
			return
		}

		s, path, err := store.ResolveReadStore(ctx, tx, f.BlobID, f.StoragePath)
		if err != nil {
			Err(c, http.StatusInternalServerError, "no active storage configured")
			return
		}
		st, err := store.BuildStorage(ctx, tx, s)
		if err != nil {
			Err(c, http.StatusInternalServerError, "building storage: "+err.Error())
			return
		}

		url, err := st.GetSignedURL(ctx, path, time.Hour)
		if err != nil {
			if errors.Is(err, storage.ErrNotSupported) {
				Err(c, http.StatusNotImplemented, "signed URLs not supported for this provider")
				return
			}
			Err(c, http.StatusInternalServerError, "signing url: "+err.Error())
			return
		}

		Success(c, gin.H{
			"url":      url,
			"filename": f.Name,
			"mimeType": f.MimeType,
			"size":     f.Size,
		})
	}
}

// StorageUsage returns the workspace's used/limit quota + counts.
func StorageUsage(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var used int64
		if err := tx.NewSelect().Model((*model.File)(nil)).
			Where("status = 'ready'").
			ColumnExpr("COALESCE(SUM(size), 0)").
			Scan(ctx, &used); err != nil {
			Err(c, http.StatusInternalServerError, "querying usage: "+err.Error())
			return
		}
		fileCount, _ := tx.NewSelect().Model((*model.File)(nil)).Where("status = 'ready'").Count(ctx)
		folderCount, _ := tx.NewSelect().Model((*model.Folder)(nil)).Count(ctx)

		// Limit is fixed at the Locker default until per-org quota wiring.
		const limit int64 = 5 << 30 // 5 GB
		percentage := 0.0
		if limit > 0 {
			percentage = float64(used) / float64(limit) * 100
		}

		Success(c, gin.H{
			"used":        used,
			"limit":       limit,
			"fileCount":   fileCount,
			"folderCount": folderCount,
			"percentage":  percentage,
		})
	}
}

// ListFiles returns files in a folder with filters, sorting and pagination.
// Dot-folder files (e.g. .plugins) are excluded. Filters: folderId, search,
// tagSlugs (comma-separated, ALL must match), fileTypes (mime prefix), sort.
func ListFiles(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 200 {
			pageSize = 50
		}

		q := tx.NewSelect().Model((*model.File)(nil)).
			Where("status = 'ready' AND name NOT LIKE '.%'")

		if folderID := c.Query("folderId"); folderID != "" {
			id, err := uuid.Parse(folderID)
			if err != nil {
				Err(c, http.StatusBadRequest, "invalid folderId")
				return
			}
			q.Where("folder_id = ?", id)
		} else {
			q.Where("folder_id IS NULL")
		}
		if s := c.Query("search"); s != "" {
			q.Where("LOWER(name) LIKE LOWER(?)", "%"+s+"%")
		}
		if ft := c.Query("fileTypes"); ft != "" {
			q.Where("LOWER(mime_type) LIKE LOWER(?)", ft+"%")
		}
		if tagSlugs := c.Query("tagSlugs"); tagSlugs != "" {
			slugs := strings.Split(tagSlugs, ",")
			sub := tx.NewSelect().Model((*model.FileTag)(nil)).
				Join("JOIN tags ON tags.id = file_tag.tag_id").
				Where("tags.slug IN (?)", bun.In(slugs)).
				ColumnExpr("DISTINCT file_tag.file_id")
			q.Where("id IN (?)", sub)
		}

		sortField := "created_at"
		switch c.Query("sort") {
		case "name":
			sortField = "name"
		case "size":
			sortField = "size"
		case "updated_at":
			sortField = "updated_at"
		}
		sortDir := "DESC"
		if c.Query("sortDir") == "asc" {
			sortDir = "ASC"
		}
		q.Order(sortField + " " + sortDir)

		total, err := q.Count(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "counting files: "+err.Error())
			return
		}
		var files []model.File
		if err := q.Offset((page-1)*pageSize).Limit(pageSize).Scan(ctx, &files); err != nil {
			Err(c, http.StatusInternalServerError, "listing files: "+err.Error())
			return
		}

		ids := make([]uuid.UUID, 0, len(files))
		for _, f := range files {
			ids = append(ids, f.ID)
		}
		tagsByFile, err := loadTagsForFiles(ctx, tx, ids)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading tags: "+err.Error())
			return
		}

		Success(c, gin.H{
			"files":    files,
			"tags":     tagsByFile,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// UpdateFile renames/moves a file. Body: { name?, folderId? }.
// When the display path changes the object is physically relocated on the
// primary store and object_key/blob_location rows are kept in sync.
func UpdateFile(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid file id")
			return
		}

		var req struct {
			Name     string  `json:"name"`
			FolderID *string `json:"folderId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}

		var f model.File
		if err := tx.NewSelect().Model(&f).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "file not found")
			return
		}

		newName := f.Name
		if n := trimSpace(req.Name); n != "" && n != f.Name {
			newName = n
		}
		var newFolder *uuid.UUID
		folderProvided := req.FolderID != nil
		if folderProvided && *req.FolderID != "" {
			fid, err := uuid.Parse(*req.FolderID)
			if err != nil {
				Err(c, http.StatusBadRequest, "invalid folderId")
				return
			}
			if err := tx.NewSelect().Model((*model.Folder)(nil)).Where("id = ?", fid).Scan(ctx, &model.Folder{}); err != nil {
				Err(c, http.StatusNotFound, "folder not found")
				return
			}
			newFolder = &fid
		}

		if err := ensureFileNameUnique(ctx, tx, newName, newFolder, id); err != nil {
			Err(c, http.StatusConflict, err.Error())
			return
		}

		// Compute the new display path / object key.
		oldKey := f.StoragePath
		newKey := oldKey
		if newName != f.Name || folderProvided {
			dir := ""
			if newFolder != nil {
				dir, err = store.FolderPath(ctx, tx, *newFolder)
				if err != nil {
					Err(c, http.StatusInternalServerError, err.Error())
					return
				}
			}
			newKey = newName
			if dir != "" {
				newKey = dir + "/" + newName
			}
		}

		// Physically relocate the object before committing the DB change so a
		// failure leaves the old record intact.
		if newKey != oldKey {
			s, path, err := store.ResolveReadStore(ctx, tx, f.BlobID, oldKey)
			if err != nil {
				Err(c, http.StatusInternalServerError, "no active storage configured")
				return
			}
			st, err := store.BuildStorage(ctx, tx, s)
			if err != nil {
				Err(c, http.StatusInternalServerError, "building storage: "+err.Error())
				return
			}
			if err := moveObject(ctx, st, path, newKey, f.MimeType); err != nil {
				Err(c, http.StatusInternalServerError, "moving file: "+err.Error())
				return
			}
		}

		updated, err := store.RenameFile(ctx, tx, id, newName, newFolder, newKey)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		Success(c, gin.H{"file": updated})
	}
}

// DeleteFile removes a file: physical objects from every store, then the
// file + blob + blob_location records (delete everywhere).
func DeleteFile(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid file id")
			return
		}

		var f model.File
		if err := tx.NewSelect().Model(&f).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "file not found")
			return
		}

		locs, err := store.BlobLocationsForFile(ctx, tx, f.BlobID)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		for _, loc := range locs {
			var s model.Store
			if err := tx.NewSelect().Model(&s).Where("id = ?", loc.StoreID).Scan(ctx); err != nil {
				continue
			}
			if st, err := store.BuildStorage(ctx, tx, &s); err == nil {
				_ = st.Delete(ctx, loc.StoragePath)
			}
		}

		if err := store.DeleteFileEverywhere(ctx, tx, id); err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		Msg(c, "file deleted")
	}
}

// SearchFiles returns files matching q by name (transcription fallback M10).
func SearchFiles(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		q := strings.TrimSpace(c.Query("q"))
		if q == "" {
			Err(c, http.StatusBadRequest, "q is required")
			return
		}
		var files []model.File
		if err := tx.NewSelect().Model((*model.File)(nil)).
			Where("status = 'ready' AND LOWER(name) LIKE LOWER(?)", "%"+q+"%").
			Order("name ASC").
			Limit(100).
			Scan(ctx, &files); err != nil {
			Err(c, http.StatusInternalServerError, "searching files: "+err.Error())
			return
		}
		ids := make([]uuid.UUID, 0, len(files))
		for _, f := range files {
			ids = append(ids, f.ID)
		}
		tagsByFile, err := loadTagsForFiles(ctx, tx, ids)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading tags: "+err.Error())
			return
		}
		Success(c, gin.H{"files": files, "tags": tagsByFile})
	}
}

// moveObject relocates an object on a storage provider: download old, upload
// new, delete old.
func moveObject(ctx context.Context, st storage.Storage, oldKey, newKey, mime string) error {
	if oldKey == newKey {
		return nil
	}
	r, _, err := st.Download(ctx, oldKey)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", oldKey, err)
	}
	defer r.Close()
	if mime == "" {
		mime = "application/octet-stream"
	}
	if err := st.Upload(ctx, newKey, r, mime); err != nil {
		return fmt.Errorf("uploading %s: %w", newKey, err)
	}
	return st.Delete(ctx, oldKey)
}

// ensureFileNameUnique rejects duplicate ready file names within a folder.
func ensureFileNameUnique(ctx context.Context, tx bun.IDB, name string, folderID *uuid.UUID, exclude uuid.UUID) error {
	q := tx.NewSelect().Model((*model.File)(nil)).
		Where("status = 'ready' AND name = ? AND id != ?", name, exclude)
	if folderID != nil {
		q.Where("folder_id = ?", *folderID)
	} else {
		q.Where("folder_id IS NULL")
	}
	n, err := q.Count(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("a file named %q already exists here", name)
	}
	return nil
}

// loadTagsForFiles returns a map of fileID → tags for the given file IDs.
func loadTagsForFiles(ctx context.Context, tx bun.IDB, fileIDs []uuid.UUID) (map[uuid.UUID][]model.Tag, error) {
	out := make(map[uuid.UUID][]model.Tag, len(fileIDs))
	if len(fileIDs) == 0 {
		return out, nil
	}
	var fts []model.FileTag
	if err := tx.NewSelect().Model(&fts).Where("file_id IN (?)", bun.In(fileIDs)).Scan(ctx); err != nil {
		return nil, err
	}
	tagIDs := make([]uuid.UUID, 0, len(fts))
	seen := map[uuid.UUID]bool{}
	for _, ft := range fts {
		if !seen[ft.TagID] {
			seen[ft.TagID] = true
			tagIDs = append(tagIDs, ft.TagID)
		}
	}
	var tags []model.Tag
	if len(tagIDs) > 0 {
		if err := tx.NewSelect().Model(&tags).Where("id IN (?)", bun.In(tagIDs)).Scan(ctx); err != nil {
			return nil, err
		}
	}
	tagByID := make(map[uuid.UUID]model.Tag, len(tags))
	for _, t := range tags {
		tagByID[t.ID] = t
	}
	for _, ft := range fts {
		if t, ok := tagByID[ft.TagID]; ok {
			out[ft.FileID] = append(out[ft.FileID], t)
		}
	}
	return out, nil
}
