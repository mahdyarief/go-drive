package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// File is a tenant-scoped file record. Content lives in the referenced
// FileBlob, which is replicated across Stores (see BlobLocation).
type File struct {
	bun.BaseModel `bun:"table:files"`

	ID              uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	UserID          string     `json:"user_id" bun:"user_id,notnull"`
	FolderID        *uuid.UUID `json:"folder_id" bun:"folder_id,type:uuid"`
	BlobID          uuid.UUID  `json:"blob_id" bun:"blob_id,type:uuid,notnull"`
	Name            string     `json:"name" bun:"name,notnull"`
	MimeType        string     `json:"mime_type" bun:"mime_type,notnull"`
	Size            int64      `json:"size" bun:"size,notnull"`
	StoragePath     string     `json:"storage_path" bun:"storage_path,notnull"`
	StorageProvider string     `json:"storage_provider" bun:"storage_provider,notnull"`
	Status          string     `json:"status" bun:"status,notnull,default:'ready'"`
	ThumbnailPath   string     `json:"thumbnail_path" bun:"thumbnail_path"`
	Checksum        string     `json:"checksum" bun:"checksum"`
	S3Key           string     `json:"s3_key" bun:"s3_key"`
	ReplacesFileID  *uuid.UUID `json:"replaces_file_id" bun:"replaces_file_id,type:uuid"`
	StorageTier     string     `json:"storage_tier" bun:"storage_tier,notnull,default:'standard'"`
	LastAccessedAt  *time.Time `json:"last_accessed_at" bun:"last_accessed_at"`
	CreatedAt       time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time  `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
