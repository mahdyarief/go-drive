package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// StoreRoutingPolicy controls how uploads are distributed across the
// workspace's write stores in cumulative mode. Single row per workspace
// (workspace_id = uuid.Nil), mirroring WorkspaceStorageSetting.
//
// Mode is one of:
//   - most_available: pick the store with the most available bytes.
//   - round_robin: cycle through eligible stores using round_robin_cursor.
//   - priority: pick the first store in priority_store_ids that can fit.
type StoreRoutingPolicy struct {
	bun.BaseModel `bun:"table:store_routing_policy"`

	WorkspaceID      uuid.UUID   `json:"workspace_id" bun:"workspace_id,pk,type:uuid"`
	Mode             string      `json:"mode" bun:"mode,notnull,default:'most_available'"`
	PriorityStoreIDs []uuid.UUID `json:"priority_store_ids" bun:"priority_store_ids,type:jsonb,notnull"`
	RoundRobinCursor int         `json:"round_robin_cursor" bun:"round_robin_cursor,notnull,default:0"`
	CreatedAt        time.Time   `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time   `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
