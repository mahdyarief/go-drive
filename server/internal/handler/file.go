package handler

import (
	"context"
	"database/sql"
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

		// Workspace limit depends on the storage mode: cumulative sums every
		// active store's quota (each file lives on one store), replicate uses
		// the primary store's quota (every file is mirrored). 0 = unlimited.
		// GDrive stores use the live provider capacity (provider_quota_limit)
		// rather than the app-configured quota_limit, which stays 0.
		var limit int64
		mode, err := store.GetStorageMode(ctx, tx)
		if err != nil {
			Err(c, http.StatusInternalServerError, err.Error())
			return
		}
		if mode == "replicate" {
			if primary, err := store.ResolvePrimaryStore(ctx, tx); err == nil {
				limit = primary.QuotaLimit
			}
		} else if err := tx.NewSelect().Model((*model.Store)(nil)).
			Where("status = 'active'").
			ColumnExpr("COALESCE(SUM(CASE WHEN provider = 'gdrive' THEN provider_quota_limit ELSE quota_limit END), 0)").
			Scan(ctx, &limit); err != nil {
			Err(c, http.StatusInternalServerError, "summing store quotas: "+err.Error())
			return
		}

		// The org's admin-allocated quota (0 = unlimited) caps the store capacity.
		// Quota only applies to local provider stores — gdrive capacity is per-org
		// assigned and not counted toward the limit.
		var allocated int64
		_ = db.NewRaw(`
			SELECT COALESCE(oq.quota_limit, 0)
			FROM org_quotas oq
			JOIN organizations o ON o.id = oq.organization_id
			WHERE o.slug = ?
		`, c.GetString("org_slug")).Scan(ctx, &allocated)
		if allocated > 0 && (limit == 0 || allocated < limit) {
			limit = allocated
		}
		// The owner's admin-assigned user limit is the hard ceiling even when
		// no explicit org allocation has been set yet.
		if userLimit, err := orgOwnerUserLimit(ctx, db, c.GetString("org_slug")); err == nil && userLimit > 0 && (limit == 0 || userLimit < limit) {
			limit = userLimit
		}
		percentage := 0.0
		if limit > 0 {
			percentage = float64(used) / float64(limit) * 100
		}

		Success(c, gin.H{
			"used":        used,
			"limit":       limit,
			"allocated":   allocated,
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
		storesByFile, err := loadStoresForFiles(ctx, tx, files)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading stores: "+err.Error())
			return
		}

		Success(c, gin.H{
			"files":    files,
			"tags":     tagsByFile,
			"stores":   storesByFile,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// RecentFiles returns the most recently updated files across all folders,
// used by the dashboard's "recent files" section. Dot-folder files are excluded.
func RecentFiles(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "8"))
		if limit < 1 || limit > 50 {
			limit = 8
		}

		var files []model.File
		if err := tx.NewSelect().Model((*model.File)(nil)).
			Where("status = 'ready' AND name NOT LIKE '.%'").
			Order("updated_at DESC").
			Limit(limit).
			Scan(ctx, &files); err != nil {
			Err(c, http.StatusInternalServerError, "listing recent files: "+err.Error())
			return
		}

		Success(c, gin.H{"files": files})
	}
}

// UpdateFile renames/moves a file. Body: { name?, folderId? }.
// When the display path changes the object is physically relocated on the
// primary store and object_key/blob_location rows are kept in sync.
func UpdateFile(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
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
		// Audit folder moves only; renames keep the same folder.
		if folderProvided && ((newFolder == nil) != (f.FolderID == nil) || (newFolder != nil && f.FolderID != nil && *newFolder != *f.FolderID)) {
			auditLog(ctx, tx, userID, "file_move", "file", id.String(), map[string]any{"name": f.Name})
		}
		Success(c, gin.H{"file": updated})
	}
}

// DeleteFile removes a file: physical objects from every store, then the
// file + blob + blob_location records (delete everywhere).
func DeleteFile(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
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
		auditLog(ctx, tx, userID, "file_delete", "file", id.String(), map[string]any{"name": f.Name})
		Msg(c, "file deleted")
	}
}

// BatchMoveFiles moves multiple files into a destination folder. The whole
// batch fails on the first error so the tenant transaction rolls back.
func BatchMoveFiles(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		var req struct {
			IDs      []string `json:"ids"`
			FolderID string   `json:"folderId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.IDs) == 0 {
			Err(c, http.StatusBadRequest, "ids is required")
			return
		}

		var newFolder *uuid.UUID
		if req.FolderID != "" {
			fid, err := uuid.Parse(req.FolderID)
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

		dir := ""
		var err error
		if newFolder != nil {
			dir, err = store.FolderPath(ctx, tx, *newFolder)
			if err != nil {
				Err(c, http.StatusInternalServerError, err.Error())
				return
			}
		}

		for _, idStr := range req.IDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				Err(c, http.StatusBadRequest, "invalid file id: "+idStr)
				return
			}
			var f model.File
			if err := tx.NewSelect().Model(&f).Where("id = ?", id).Scan(ctx); err != nil {
				Err(c, http.StatusNotFound, "file not found: "+idStr)
				return
			}
			if err := ensureFileNameUnique(ctx, tx, f.Name, newFolder, id); err != nil {
				Err(c, http.StatusConflict, err.Error())
				return
			}
			newKey := f.Name
			if dir != "" {
				newKey = dir + "/" + f.Name
			}
			if newKey != f.StoragePath {
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
				if err := moveObject(ctx, st, path, newKey, f.MimeType); err != nil {
					Err(c, http.StatusInternalServerError, "moving file: "+err.Error())
					return
				}
			}
			if _, err := store.RenameFile(ctx, tx, id, f.Name, newFolder, newKey); err != nil {
				Err(c, http.StatusInternalServerError, err.Error())
				return
			}
		}

		auditLog(ctx, tx, userID, "file_move_batch", "file", "", map[string]any{"count": len(req.IDs)})
		Success(c, gin.H{"moved": len(req.IDs)})
	}
}

// BatchDeleteFiles removes multiple files. Missing files are skipped so the
// operation is idempotent.
func BatchDeleteFiles(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		var req struct {
			IDs []string `json:"ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.IDs) == 0 {
			Err(c, http.StatusBadRequest, "ids is required")
			return
		}

		deleted := 0
		for _, idStr := range req.IDs {
			id, err := uuid.Parse(idStr)
			if err != nil {
				Err(c, http.StatusBadRequest, "invalid file id: "+idStr)
				return
			}
			var f model.File
			if err := tx.NewSelect().Model(&f).Where("id = ?", id).Scan(ctx); err != nil {
				continue
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
			deleted++
		}

		auditLog(ctx, tx, userID, "file_delete_batch", "file", "", map[string]any{"count": deleted})
		Success(c, gin.H{"deleted": deleted})
	}
}

// SearchFiles returns files matching q by name (transcription fallback M10).
// GetFile returns a single file by ID. The preview page uses this endpoint so
// it survives hard refreshes instead of relying on navigation state.
func GetFile(db *bun.DB) gin.HandlerFunc {
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
			if errors.Is(err, sql.ErrNoRows) {
				Err(c, http.StatusNotFound, "file not found")
				return
			}
			Err(c, http.StatusInternalServerError, "loading file: "+err.Error())
			return
		}

		Success(c, gin.H{"file": f})
	}
}

func SearchFiles(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		q := strings.TrimSpace(c.Query("q"))
		if q == "" {
			Err(c, http.StatusBadRequest, "q is required")
			return
		}
		qb := tx.NewSelect().Model((*model.File)(nil)).
			Where("status = 'ready' AND LOWER(name) LIKE LOWER(?)", "%"+q+"%")
		if kind := strings.TrimSpace(c.Query("kind")); kind != "" {
			qb = qb.Where("LOWER(mime_type) LIKE LOWER(?)", kind+"%")
		}
		if v := strings.TrimSpace(c.Query("minSize")); v != "" {
			if min, err := strconv.ParseInt(v, 10, 64); err == nil && min >= 0 {
				qb = qb.Where("size >= ?", min)
			}
		}
		if v := strings.TrimSpace(c.Query("maxSize")); v != "" {
			if max, err := strconv.ParseInt(v, 10, 64); err == nil && max >= 0 {
				qb = qb.Where("size <= ?", max)
			}
		}
		if v := strings.TrimSpace(c.Query("from")); v != "" {
			if from, err := time.Parse("2006-01-02", v); err == nil {
				qb = qb.Where("created_at >= ?", from)
			}
		}
		if v := strings.TrimSpace(c.Query("to")); v != "" {
			if to, err := time.Parse("2006-01-02", v); err == nil {
				qb = qb.Where("created_at < ?", to.AddDate(0, 0, 1))
			}
		}
		var files []model.File
		if err := qb.Order("name ASC").Limit(100).Scan(ctx, &files); err != nil {
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
	if mime == "" {
		mime = "application/octet-stream"
	}
	if err := st.Upload(ctx, newKey, r, mime); err != nil {
		r.Close()
		return fmt.Errorf("uploading %s: %w", newKey, err)
	}
	// Close the source handle before Delete: on Windows os.Remove fails while
	// the file is still open ("being used by another process").
	if err := r.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", oldKey, err)
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

// fileStoreInfo is a lightweight store reference attached to each file in list
// responses so the UI can show which storage holds the file's blob. A blob can
// live on multiple stores in replicate mode; cumulative mode yields one.
type fileStoreInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// loadStoresForFiles returns file_id -> stores holding the file's blob via the
// blob_locations join. Only locations with state 'available' are reported.
func loadStoresForFiles(ctx context.Context, tx bun.IDB, files []model.File) (map[uuid.UUID][]fileStoreInfo, error) {
	out := make(map[uuid.UUID][]fileStoreInfo, len(files))
	if len(files) == 0 {
		return out, nil
	}
	blobIDs := make([]uuid.UUID, 0, len(files))
	blobToFile := make(map[uuid.UUID]uuid.UUID, len(files))
	for _, f := range files {
		if _, ok := blobToFile[f.BlobID]; ok {
			continue
		}
		blobToFile[f.BlobID] = f.ID
		blobIDs = append(blobIDs, f.BlobID)
	}
	var rows []struct {
		BlobID   uuid.UUID `bun:"blob_id"`
		StoreID  uuid.UUID `bun:"store_id"`
		Name     string    `bun:"name"`
		Provider string    `bun:"provider"`
	}
	if err := tx.NewSelect().
		TableExpr("blob_locations AS bl").
		ColumnExpr("bl.blob_id AS blob_id, s.id AS store_id, s.name AS name, s.provider AS provider").
		Join("JOIN stores AS s ON s.id = bl.store_id").
		Where("bl.blob_id IN (?)", bun.In(blobIDs)).
		Where("bl.state = 'available'").
		Scan(ctx, &rows); err != nil {
		return nil, err
	}
	for _, r := range rows {
		fileID, ok := blobToFile[r.BlobID]
		if !ok {
			continue
		}
		out[fileID] = append(out[fileID], fileStoreInfo{ID: r.StoreID.String(), Name: r.Name, Provider: r.Provider})
	}
	return out, nil
}
