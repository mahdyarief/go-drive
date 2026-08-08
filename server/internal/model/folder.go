package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Folder is a tenant-scoped folder node in the file explorer tree.
// Workspace scoping is implicit via the tenant schema (search_path).
type Folder struct {
	bun.BaseModel `bun:"table:folders"`

	ID        uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	UserID    string     `json:"user_id" bun:"user_id,notnull"`
	ParentID  *uuid.UUID `json:"parent_id" bun:"parent_id,type:uuid"`
	Name      string     `json:"name" bun:"name,notnull"`
	Color     string     `json:"color" bun:"color"`
	CreatedAt time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time  `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
