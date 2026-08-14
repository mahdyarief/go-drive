package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
	"go-drive/server/internal/store"
)

// ListFolders returns folders for a parent (or root when parentId is empty).
// Dot-folders (e.g. .plugins) are excluded from the explorer listing.
func ListFolders(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		q := tx.NewSelect().Model((*model.Folder)(nil)).
			Where("name NOT LIKE '.%'").
			Order("name ASC")
		if p := c.Query("parentId"); p != "" {
			id, err := uuid.Parse(p)
			if err != nil {
				Err(c, http.StatusBadRequest, "invalid parentId")
				return
			}
			q.Where("parent_id = ?", id)
		} else {
			q.Where("parent_id IS NULL")
		}

		var folders []model.Folder
		if err := q.Scan(ctx, &folders); err != nil {
			Err(c, http.StatusInternalServerError, "listing folders: "+err.Error())
			return
		}
		Success(c, gin.H{"folders": folders})
	}
}

// RecentFolders returns the most recently updated folders in the tenant.
// Dot-folders (e.g. .plugins) are excluded, matching ListFolders.
func RecentFolders(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		limit := 4
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}

		var folders []model.Folder
		if err := tx.NewSelect().Model((*model.Folder)(nil)).
			Where("name NOT LIKE '.%'").
			Order("updated_at DESC").
			Limit(limit).
			Scan(ctx, &folders); err != nil {
			Err(c, http.StatusInternalServerError, "listing recent folders: "+err.Error())
			return
		}
		Success(c, gin.H{"folders": folders})
	}
}

// CreateFolder creates a folder. Body: { name, parentId?, color? }.
func CreateFolder(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		var req struct {
			Name     string     `json:"name"`
			ParentID *uuid.UUID `json:"parentId"`
			Color    string     `json:"color"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		req.Name = trimSpace(req.Name)
		if req.Name == "" {
			Err(c, http.StatusBadRequest, "name is required")
			return
		}
		if req.ParentID != nil {
			if err := folderExists(ctx, tx, *req.ParentID); err != nil {
				Err(c, http.StatusNotFound, "parent folder not found")
				return
			}
		}
		if err := ensureFolderNameUnique(ctx, tx, req.Name, req.ParentID, uuid.Nil); err != nil {
			Err(c, http.StatusConflict, err.Error())
			return
		}

		f := &model.Folder{
			ID:       uuid.New(),
			UserID:   userID,
			ParentID: req.ParentID,
			Name:     req.Name,
			Color:    req.Color,
		}
		if _, err := tx.NewInsert().Model(f).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "creating folder: "+err.Error())
			return
		}
		auditLog(ctx, tx, userID, "folder_create", "folder", f.ID.String(), map[string]any{"name": f.Name})
		Created(c, gin.H{"folder": f})
	}
}

// FolderBreadcrumbs returns the ancestor chain (root → leaf) for a folder.
func FolderBreadcrumbs(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var crumbs []gin.H
		if p := c.Query("folderId"); p != "" {
			id, err := uuid.Parse(p)
			if err != nil {
				Err(c, http.StatusBadRequest, "invalid folderId")
				return
			}
			var parts []gin.H
			cur := &id
			for cur != nil {
				var f model.Folder
				if err := tx.NewSelect().Model(&f).Where("id = ?", *cur).Scan(ctx); err != nil {
					break
				}
				parts = append([]gin.H{{"id": f.ID, "name": f.Name}}, parts...)
				cur = f.ParentID
				if len(parts) > 50 {
					break
				}
			}
			crumbs = parts
		}
		Success(c, gin.H{"breadcrumbs": crumbs})
	}
}

// UpdateFolder renames/moves a folder. Body: { name?, parentId?, color? }.
// Moving is guarded against cycles (a folder cannot be its own descendant).
func UpdateFolder(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid folder id")
			return
		}

		var req struct {
			Name     string     `json:"name"`
			ParentID *uuid.UUID `json:"parentId"`
			Color    string     `json:"color"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}

		var f model.Folder
		if err := tx.NewSelect().Model(&f).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "folder not found")
			return
		}

		name := trimSpace(req.Name)
		if name == "" {
			name = f.Name
		}
		newParent := f.ParentID
		if req.ParentID != nil {
			if *req.ParentID == id {
				Err(c, http.StatusBadRequest, "folder cannot be its own parent")
				return
			}
			if err := folderExists(ctx, tx, *req.ParentID); err != nil {
				Err(c, http.StatusNotFound, "parent folder not found")
				return
			}
			if isDescendant(ctx, tx, id, *req.ParentID) {
				Err(c, http.StatusBadRequest, "cannot move folder into its own subtree")
				return
			}
			newParent = req.ParentID
		}
		if name != f.Name || newParent != f.ParentID {
			if err := ensureFolderNameUnique(ctx, tx, name, newParent, id); err != nil {
				Err(c, http.StatusConflict, err.Error())
				return
			}
		}

		u := tx.NewUpdate().Model((*model.Folder)(nil))
		if name != f.Name {
			u.Set("name = ?", name)
		}
		if newParent != f.ParentID {
			u.Set("parent_id = ?", newParent)
		}
		if req.Color != "" && req.Color != f.Color {
			u.Set("color = ?", req.Color)
		}
		if _, err := u.Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "updating folder: "+err.Error())
			return
		}

		var updated model.Folder
		if err := tx.NewSelect().Model(&updated).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "reloading folder: "+err.Error())
			return
		}
		Success(c, gin.H{"folder": updated})
	}
}

// DeleteFolder removes a folder and all descendants; files inside move to root.
// Query param with_files=true deletes files from storage + DB instead.
func DeleteFolder(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		userID := c.GetString("user_id")
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid folder id")
			return
		}
		var f model.Folder
		if err := tx.NewSelect().Model(&f).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "folder not found")
			return
		}

		withFiles := c.Query("with_files") == "true"

		if withFiles {
			foldersDeleted, filesDeleted, err := store.DeleteFolderRecursiveWithFiles(ctx, tx, id)
			if err != nil {
				Err(c, http.StatusInternalServerError, err.Error())
				return
			}
			auditLog(ctx, tx, userID, "folder_delete_with_files", "folder", id.String(), map[string]any{"name": f.Name, "files_deleted": filesDeleted})
			Success(c, gin.H{"folders_deleted": foldersDeleted, "files_deleted": filesDeleted})
		} else {
			n, err := store.DeleteFolderRecursive(ctx, tx, id)
			if err != nil {
				Err(c, http.StatusInternalServerError, err.Error())
				return
			}
			auditLog(ctx, tx, userID, "folder_delete", "folder", id.String(), map[string]any{"name": f.Name})
			Success(c, gin.H{"deleted": n})
		}
	}
}

func folderExists(ctx context.Context, tx bun.IDB, id uuid.UUID) error {
	return tx.NewSelect().Model((*model.Folder)(nil)).Where("id = ?", id).Scan(ctx, &model.Folder{})
}

func trimSpace(s string) string {
	var start, end int
	for start = 0; start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r'); start++ {
	}
	for end = len(s); end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r'); end-- {
	}
	return s[start:end]
}

// ensureFolderNameUnique rejects duplicate folder names within a parent.
func ensureFolderNameUnique(ctx context.Context, tx bun.IDB, name string, parentID *uuid.UUID, exclude uuid.UUID) error {
	q := tx.NewSelect().Model((*model.Folder)(nil)).Where("name = ? AND id != ?", name, exclude)
	if parentID != nil {
		q.Where("parent_id = ?", *parentID)
	} else {
		q.Where("parent_id IS NULL")
	}
	n, err := q.Count(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("a folder named %q already exists here", name)
	}
	return nil
}

// isDescendant reports whether target is inside start's subtree.
func isDescendant(ctx context.Context, tx bun.IDB, start, target uuid.UUID) bool {
	ids, err := store.FolderSubtree(ctx, tx, start)
	if err != nil {
		return true // fail closed on error
	}
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
