package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// FilePreviewToken is a short-lived token that grants anonymous access to a
// ready file (see GET /api/preview/:token). Only the SHA-256 hash of the raw
// token is stored; the raw token is returned to the creator exactly once.
type FilePreviewToken struct {
	bun.BaseModel `bun:"table:file_preview_tokens"`

	ID        uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	FileID    uuid.UUID `json:"file_id" bun:"file_id,type:uuid,notnull"`
	UserID    string    `json:"user_id" bun:"user_id,notnull"`
	TokenHash string    `json:"token_hash" bun:"token_hash,notnull,unique"`
	ExpiresAt time.Time `json:"expires_at" bun:"expires_at,notnull"`
	CreatedAt time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
