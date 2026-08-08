package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// FileBlob is the content-addressable blob record backing a File.
// object_key is the human-readable display path (e.g. docs/reports/q1.pdf).
type FileBlob struct {
	bun.BaseModel `bun:"table:file_blobs"`

	ID          uuid.UUID      `json:"id" bun:"id,pk,type:uuid"`
	CreatedByID string         `json:"created_by_id" bun:"created_by_id"`
	ObjectKey   string         `json:"object_key" bun:"object_key,notnull"`
	ByteSize    int64          `json:"byte_size" bun:"byte_size,notnull"`
	MimeType    string         `json:"mime_type" bun:"mime_type,notnull"`
	Checksum    string         `json:"checksum" bun:"checksum"`
	State       string         `json:"state" bun:"state,notnull,default:'pending'"`
	Metadata    map[string]any `json:"metadata" bun:"metadata,type:jsonb"`
	CreatedAt   time.Time      `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt   time.Time      `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// BlobLocation records which Store holds a copy of a FileBlob.
type BlobLocation struct {
	bun.BaseModel `bun:"table:blob_locations"`

	ID             uuid.UUID      `json:"id" bun:"id,pk,type:uuid"`
	BlobID         uuid.UUID      `json:"blob_id" bun:"blob_id,type:uuid,notnull"`
	StoreID        uuid.UUID      `json:"store_id" bun:"store_id,type:uuid,notnull"`
	StoragePath    string         `json:"storage_path" bun:"storage_path,notnull"`
	State          string         `json:"state" bun:"state,notnull,default:'pending'"`
	Origin         string         `json:"origin" bun:"origin,notnull"`
	LastVerifiedAt *time.Time     `json:"last_verified_at" bun:"last_verified_at"`
	LastError      string         `json:"last_error" bun:"last_error"`
	Metadata       map[string]any `json:"metadata" bun:"metadata,type:jsonb"`
	CreatedAt      time.Time      `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time      `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
