package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Store is a storage backend attached to a workspace (Local, S3-compatible,
// or Google Drive). Provider enum adds 'gdrive' over Locker's original set.
type Store struct {
	bun.BaseModel `bun:"table:stores"`

	ID               uuid.UUID      `json:"id" bun:"id,pk,type:uuid"`
	Name             string         `json:"name" bun:"name,notnull"`
	Provider         string         `json:"provider" bun:"provider,notnull"`
	CredentialSource string         `json:"credential_source" bun:"credential_source,notnull,default:'store'"`
	Status           string         `json:"status" bun:"status,notnull,default:'active'"`
	WriteMode        string         `json:"write_mode" bun:"write_mode,notnull,default:'write'"`
	IngestMode       string         `json:"ingest_mode" bun:"ingest_mode,notnull,default:'none'"`
	ReadPriority     int            `json:"read_priority" bun:"read_priority,notnull,default:100"`
	Config           map[string]any `json:"config" bun:"config,type:jsonb,notnull"`
	QuotaUsed        int64          `json:"quota_used" bun:"quota_used,notnull,default:0"`
	QuotaLimit       int64          `json:"quota_limit" bun:"quota_limit,notnull,default:0"`
	LastTestedAt     *time.Time     `json:"last_tested_at" bun:"last_tested_at"`
	LastSyncedAt     *time.Time     `json:"last_synced_at" bun:"last_synced_at"`
	CreatedAt        time.Time      `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time      `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// StoreSecret holds the encrypted credentials for a Store (1:1).
type StoreSecret struct {
	bun.BaseModel `bun:"table:store_secrets"`

	StoreID              uuid.UUID `json:"store_id" bun:"store_id,pk,type:uuid"`
	EncryptionVersion    int       `json:"encryption_version" bun:"encryption_version,notnull,default:1"`
	EncryptedCredentials string    `json:"encrypted_credentials" bun:"encrypted_credentials,notnull"`
	CreatedAt            time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt            time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// WorkspaceStorageSetting points a workspace at its primary store (1:1) and
// carries the global storage mode: 'replicate' (master/slave fan-out) or
// 'cumulative' (distributed — each file lives in one store, total capacity
// is the sum of all store quotas).
type WorkspaceStorageSetting struct {
	bun.BaseModel `bun:"table:workspace_storage_settings"`

	WorkspaceID    uuid.UUID `json:"workspace_id" bun:"workspace_id,pk,type:uuid"`
	PrimaryStoreID uuid.UUID `json:"primary_store_id" bun:"primary_store_id,type:uuid,notnull"`
	StorageMode    string    `json:"storage_mode" bun:"storage_mode,notnull,default:'cumulative'"`
	CreatedAt      time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
