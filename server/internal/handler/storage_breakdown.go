package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

// StorageBreakdown returns the tenant's active-file storage usage grouped by
// category (photo/video/document), derived from mime_type. Consistent with
// StorageUsage, it is scoped by file status only (workspace-wide, not per-user).
func StorageBreakdown(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := c.MustGet("tenant_tx").(bun.Tx)
		ctx := c.Request.Context()

		var rows []struct {
			Kind  string `bun:"kind"`
			Bytes int64  `bun:"bytes"`
		}
		if err := tx.NewRaw(`
			SELECT CASE
				WHEN mime_type LIKE 'image/%' THEN 'photo'
				WHEN mime_type LIKE 'video/%' THEN 'video'
				ELSE 'document'
			END AS kind,
			COALESCE(SUM(size), 0) AS bytes
			FROM files
			WHERE status = 'ready'
			GROUP BY kind
		`).Scan(ctx, &rows); err != nil {
			Err(c, http.StatusInternalServerError, "querying storage breakdown: "+err.Error())
			return
		}

		breakdown := map[string]int64{
			"photo":    0,
			"video":    0,
			"document": 0,
		}
		var total int64
		for _, row := range rows {
			breakdown[row.Kind] = row.Bytes
			total += row.Bytes
		}

		Success(c, gin.H{
			"breakdown": gin.H{
				"photo":    breakdown["photo"],
				"video":    breakdown["video"],
				"document": breakdown["document"],
				"total":    total,
			},
		})
	}
}
