package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Tag is a workspace-scoped label that can be applied to files.
type Tag struct {
	bun.BaseModel `bun:"table:tags"`

	ID        uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	Name      string    `json:"name" bun:"name,notnull"`
	Slug      string    `json:"slug" bun:"slug,notnull"`
	Color     string    `json:"color" bun:"color"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// FileTag is the join between Files and Tags.
type FileTag struct {
	bun.BaseModel `bun:"table:file_tags"`

	ID        uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	FileID    uuid.UUID `json:"file_id" bun:"file_id,type:uuid,notnull"`
	TagID     uuid.UUID `json:"tag_id" bun:"tag_id,type:uuid,notnull"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
