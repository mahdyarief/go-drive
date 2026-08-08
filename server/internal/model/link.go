package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// ShareLink is a token-authenticated public link to a file or folder.
type ShareLink struct {
	bun.BaseModel `bun:"table:share_links"`

	ID             uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	UserID         string     `json:"user_id" bun:"user_id,notnull"`
	FileID         *uuid.UUID `json:"file_id" bun:"file_id,type:uuid"`
	FolderID       *uuid.UUID `json:"folder_id" bun:"folder_id,type:uuid"`
	Token          string     `json:"token" bun:"token,notnull,unique"`
	Access         string     `json:"access" bun:"access,notnull,default:'download'"`
	HasPassword    bool       `json:"has_password" bun:"has_password,notnull,default:false"`
	PasswordHash   string     `json:"password_hash" bun:"password_hash"`
	ExpiresAt      *time.Time `json:"expires_at" bun:"expires_at"`
	MaxDownloads   *int       `json:"max_downloads" bun:"max_downloads"`
	DownloadCount  int        `json:"download_count" bun:"download_count,notnull,default:0"`
	IsActive       bool       `json:"is_active" bun:"is_active,notnull,default:true"`
	LastAccessedAt *time.Time `json:"last_accessed_at" bun:"last_accessed_at"`
	CreatedAt      time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// UploadLink lets anonymous users upload files into a folder with constraints.
type UploadLink struct {
	bun.BaseModel `bun:"table:upload_links"`

	ID               uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	UserID           string     `json:"user_id" bun:"user_id,notnull"`
	FolderID         *uuid.UUID `json:"folder_id" bun:"folder_id,type:uuid"`
	Token            string     `json:"token" bun:"token,notnull,unique"`
	Name             string     `json:"name" bun:"name,notnull"`
	MaxFiles         *int       `json:"max_files" bun:"max_files"`
	MaxFileSize      *int64     `json:"max_file_size" bun:"max_file_size"`
	AllowedMimeTypes []string   `json:"allowed_mime_types" bun:"allowed_mime_types,type:jsonb"`
	FilesUploaded    int        `json:"files_uploaded" bun:"files_uploaded,notnull,default:0"`
	HasPassword      bool       `json:"has_password" bun:"has_password,notnull,default:false"`
	PasswordHash     string     `json:"password_hash" bun:"password_hash"`
	ExpiresAt        *time.Time `json:"expires_at" bun:"expires_at"`
	IsActive         bool       `json:"is_active" bun:"is_active,notnull,default:true"`
	CreatedAt        time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time  `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
