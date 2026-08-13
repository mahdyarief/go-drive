package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// UserQuota is the storage limit (bytes) an admin assigns to a user.
// 0 = unlimited. Lives in the public schema — users are global across orgs.
type UserQuota struct {
	bun.BaseModel `bun:"table:user_quotas"`

	UserID    string    `json:"user_id" bun:"user_id,pk"`
	QuotaLimit int64     `json:"quota_limit" bun:"quota_limit,notnull,default:0"`
	UpdatedBy string    `json:"updated_by" bun:"updated_by"`
	UpdatedAt time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// OrgQuota is the slice of a user's quota allocated to one of their orgs.
// 0 = unlimited. The sum of a user's org quotas must not exceed their
// UserQuota limit.
type OrgQuota struct {
	bun.BaseModel `bun:"table:org_quotas"`

	OrganizationID uuid.UUID `json:"organization_id" bun:"organization_id,pk,type:uuid"`
	OwnerUserID    string    `json:"owner_user_id" bun:"owner_user_id,notnull"`
	QuotaLimit     int64     `json:"quota_limit" bun:"quota_limit,notnull,default:0"`
	UpdatedAt      time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
