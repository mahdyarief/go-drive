package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// ReplicationRun tracks a replication/ingest job over a set of blobs.
type ReplicationRun struct {
	bun.BaseModel `bun:"table:replication_runs"`

	ID                uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	Kind              string     `json:"kind" bun:"kind,notnull"`
	Status            string     `json:"status" bun:"status,notnull,default:'queued'"`
	SourceStoreID     *uuid.UUID `json:"source_store_id" bun:"source_store_id,type:uuid"`
	TargetStoreID     *uuid.UUID `json:"target_store_id" bun:"target_store_id,type:uuid"`
	TriggeredByUserID string     `json:"triggered_by_user_id" bun:"triggered_by_user_id"`
	TotalItems        int        `json:"total_items" bun:"total_items,notnull,default:0"`
	ProcessedItems    int        `json:"processed_items" bun:"processed_items,notnull,default:0"`
	FailedItems       int        `json:"failed_items" bun:"failed_items,notnull,default:0"`
	ErrorMessage      string     `json:"error_message" bun:"error_message"`
	StartedAt         *time.Time `json:"started_at" bun:"started_at"`
	CompletedAt       *time.Time `json:"completed_at" bun:"completed_at"`
	CreatedAt         time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time  `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// ReplicationRunItem is one blob copy within a replication run.
type ReplicationRunItem struct {
	bun.BaseModel `bun:"table:replication_run_items"`

	ID            uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	RunID         uuid.UUID  `json:"run_id" bun:"run_id,type:uuid,notnull"`
	BlobID        uuid.UUID  `json:"blob_id" bun:"blob_id,type:uuid,notnull"`
	SourceStoreID *uuid.UUID `json:"source_store_id" bun:"source_store_id,type:uuid"`
	TargetStoreID uuid.UUID  `json:"target_store_id" bun:"target_store_id,type:uuid,notnull"`
	Status        string     `json:"status" bun:"status,notnull,default:'pending'"`
	AttemptCount  int        `json:"attempt_count" bun:"attempt_count,notnull,default:0"`
	ErrorMessage  string     `json:"error_message" bun:"error_message"`
	StartedAt     *time.Time `json:"started_at" bun:"started_at"`
	CompletedAt   *time.Time `json:"completed_at" bun:"completed_at"`
	CreatedAt     time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// IngestTombstone marks an external path as intentionally ignored during ingest.
type IngestTombstone struct {
	bun.BaseModel `bun:"table:ingest_tombstones"`

	ID              uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	StoreID         uuid.UUID  `json:"store_id" bun:"store_id,type:uuid,notnull"`
	ExternalPath    string     `json:"external_path" bun:"external_path,notnull"`
	DeletedBlobID   *uuid.UUID `json:"deleted_blob_id" bun:"deleted_blob_id,type:uuid"`
	DeletedByUserID string     `json:"deleted_by_user_id" bun:"deleted_by_user_id"`
	Reason          string     `json:"reason" bun:"reason,notnull,default:'user_deleted'"`
	CreatedAt       time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
