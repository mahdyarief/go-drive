package handler

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"
)

// Pagination represents parsed pagination parameters
type Pagination struct {
	Page     int
	PageSize int
	Offset   int
}

// ParsePagination extracts page and pageSize from query params with defaults
func ParsePagination(c *gin.Context) Pagination {
	page := 1
	pageSize := 50

	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	if ps := c.Query("pageSize"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 {
			pageSize = n
		}
	}

	// Clamp pageSize to reasonable limits
	if pageSize > 200 {
		pageSize = 200
	}
	if pageSize < 1 {
		pageSize = 1
	}

	offset := (page - 1) * pageSize

	return Pagination{
		Page:     page,
		PageSize: pageSize,
		Offset:   offset,
	}
}

// PaginatedResponse returns a standard paginated response
func PaginatedResponse(c *gin.Context, key string, items interface{}, total int, p Pagination) {
	Success(c, gin.H{
		key:      items,
		"total":  total,
		"page":   p.Page,
		"pageSize": p.PageSize,
	})
}

// CountQuery executes a count query and returns the result
func CountQuery(ctx context.Context, q *bun.SelectQuery) (int, error) {
	return q.Count(ctx)
}
