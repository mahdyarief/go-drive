package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// AuditLog records a user action within a tenant. Writes are best-effort and
// never fail the main operation; the listing is filtered to the acting user.
type AuditLog struct {
	bun.BaseModel `bun:"table:audit_logs"`

	ID         uuid.UUID      `json:"id" bun:"id,pk,type:uuid"`
	UserID     string         `json:"user_id" bun:"user_id,notnull"`
	Action     string         `json:"action" bun:"action,notnull"`
	EntityType string         `json:"entity_type" bun:"entity_type,notnull"`
	EntityID   string         `json:"entity_id" bun:"entity_id"`
	Metadata   map[string]any `json:"metadata" bun:"metadata,type:jsonb"`
	CreatedAt  time.Time      `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
