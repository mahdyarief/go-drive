package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// S3APIKey authenticates requests to the S3-compatible gateway.
type S3APIKey struct {
	bun.BaseModel `bun:"table:s3_api_keys"`

	ID              uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	UserID          string     `json:"user_id" bun:"user_id,notnull"`
	AccessKeyID     string     `json:"access_key_id" bun:"access_key_id,notnull,unique"`
	EncryptedSecret string     `json:"encrypted_secret" bun:"encrypted_secret,notnull"`
	Name            string     `json:"name" bun:"name,notnull"`
	Permissions     string     `json:"permissions" bun:"permissions,notnull,default:'readwrite'"`
	IsActive        bool       `json:"is_active" bun:"is_active,notnull,default:true"`
	LastUsedAt      *time.Time `json:"last_used_at" bun:"last_used_at"`
	ExpiresAt       *time.Time `json:"expires_at" bun:"expires_at"`
	CreatedAt       time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// S3MultipartUpload tracks an in-progress multipart upload via the S3 gateway.
type S3MultipartUpload struct {
	bun.BaseModel `bun:"table:s3_multipart_uploads"`

	ID          uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	UploadID    string    `json:"upload_id" bun:"upload_id,notnull,unique"`
	S3Key       string    `json:"s3_key" bun:"s3_key,notnull"`
	StoragePath string    `json:"storage_path" bun:"storage_path,notnull"`
	ContentType string    `json:"content_type" bun:"content_type,notnull"`
	UserID      string    `json:"user_id" bun:"user_id,notnull"`
	Status      string    `json:"status" bun:"status,notnull,default:'in_progress'"`
	CreatedAt   time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// S3MultipartPart is one uploaded part of a multipart upload.
type S3MultipartPart struct {
	bun.BaseModel `bun:"table:s3_multipart_parts"`

	ID          uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	UploadID    string    `json:"upload_id" bun:"upload_id,notnull"`
	PartNumber  int       `json:"part_number" bun:"part_number,notnull"`
	StoragePath string    `json:"storage_path" bun:"storage_path,notnull"`
	Size        int64     `json:"size" bun:"size,notnull,default:0"`
	ETag        string    `json:"etag" bun:"etag,notnull"`
	CreatedAt   time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
