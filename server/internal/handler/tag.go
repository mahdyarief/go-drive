package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// ListTags returns all workspace tags ordered by name.
func ListTags(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var tags []model.Tag
		if err := tx.NewSelect().Model(&tags).Order("name ASC").Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "listing tags: "+err.Error())
			return
		}
		Success(c, gin.H{"tags": tags})
	}
}

// CreateTag creates a tag. Body: { name, color? }.
func CreateTag(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
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
		slug := slugify(req.Name)
		if slug == "" {
			Err(c, http.StatusBadRequest, "name has no slugifiable characters")
			return
		}

		t := &model.Tag{
			ID:    uuid.New(),
			Name:  req.Name,
			Slug:  slug,
			Color: req.Color,
		}
		if _, err := tx.NewInsert().Model(t).Exec(ctx); err != nil {
			Err(c, http.StatusConflict, "a tag with this name already exists")
			return
		}
		Created(c, gin.H{"tag": t})
	}
}

// UpdateTag renames/recolors a tag. Body: { name?, color? }.
func UpdateTag(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid tag id")
			return
		}
		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}

		var t model.Tag
		if err := tx.NewSelect().Model(&t).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "tag not found")
			return
		}
		u := tx.NewUpdate().Model((*model.Tag)(nil))
		if n := trimSpace(req.Name); n != "" && n != t.Name {
			u.Set("name = ?", n, "slug = ?", slugify(n))
		}
		if req.Color != "" && req.Color != t.Color {
			u.Set("color = ?", req.Color)
		}
		if _, err := u.Where("id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusConflict, "a tag with this name already exists")
			return
		}
		var updated model.Tag
		if err := tx.NewSelect().Model(&updated).Where("id = ?", id).Scan(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "reloading tag: "+err.Error())
			return
		}
		Success(c, gin.H{"tag": updated})
	}
}

// DeleteTag removes a tag and its file_tags join rows.
func DeleteTag(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			Err(c, http.StatusBadRequest, "invalid tag id")
			return
		}
		if _, err := tx.NewDelete().Model((*model.FileTag)(nil)).Where("tag_id = ?", id).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "clearing file tags: "+err.Error())
			return
		}
		res, err := tx.NewDelete().Model((*model.Tag)(nil)).Where("id = ?", id).Exec(ctx)
		if err != nil {
			Err(c, http.StatusInternalServerError, "deleting tag: "+err.Error())
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			Err(c, http.StatusNotFound, "tag not found")
			return
		}
		Msg(c, "tag deleted")
	}
}

// SetFileTags replaces the tags on a file. Body: { fileId, tagIds }.
func SetFileTags(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var req struct {
			FileID uuid.UUID   `json:"fileId"`
			TagIDs []uuid.UUID `json:"tagIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		var f model.File
		if err := tx.NewSelect().Model(&f).Where("id = ?", req.FileID).Scan(ctx); err != nil {
			Err(c, http.StatusNotFound, "file not found")
			return
		}
		if _, err := tx.NewDelete().Model((*model.FileTag)(nil)).Where("file_id = ?", req.FileID).Exec(ctx); err != nil {
			Err(c, http.StatusInternalServerError, "clearing file tags: "+err.Error())
			return
		}
		for _, tid := range req.TagIDs {
			ft := &model.FileTag{
				ID:     uuid.New(),
				FileID: req.FileID,
				TagID:  tid,
			}
			if _, err := tx.NewInsert().Model(ft).Exec(ctx); err != nil {
				Err(c, http.StatusBadRequest, "invalid tagId: "+tid.String())
				return
			}
		}
		Success(c, gin.H{"fileId": req.FileID, "tagIds": req.TagIDs})
	}
}

// TagsForFiles returns the tags for each of the given files.
// Body: { fileIds } → { fileId: [tags] }.
func TagsForFiles(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var req struct {
			FileIDs []uuid.UUID `json:"fileIds"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Err(c, http.StatusBadRequest, "invalid request body")
			return
		}
		byFile, err := loadTagsForFiles(ctx, tx, req.FileIDs)
		if err != nil {
			Err(c, http.StatusInternalServerError, "loading tags: "+err.Error())
			return
		}
		Success(c, gin.H{"tags": byFile})
	}
}

// slugify converts a tag name to a URL-safe slug (lowercase, dashes).
func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
