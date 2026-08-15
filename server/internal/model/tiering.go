package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// TieringPolicy defines automatic storage tier transitions for a tenant.
// Files not accessed for TierDownAfterDays are moved to a cheaper tier.
type TieringPolicy struct {
	bun.BaseModel `bun:"table:tiering_policies"`

	ID                uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	Enabled           bool      `json:"enabled" bun:"enabled,notnull,default:false"`
	TierDownAfterDays int       `json:"tier_down_after_days" bun:"tier_down_after_days,notnull,default:90"`
	TierUpOnAccess    bool      `json:"tier_up_on_access" bun:"tier_up_on_access,notnull,default:true"`
	DefaultTier       string    `json:"default_tier" bun:"default_tier,notnull,default:'standard'"`
	CreatedAt         time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
