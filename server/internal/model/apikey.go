package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// APIKey authenticates external clients against the public upload API
// (POST /api/v1/uploads). It lives in the PUBLIC schema so the API-key
// middleware can look the key up by SHA-256 hash and resolve its owning org
// BEFORE opening any tenant transaction — public routes carry no session and
// no X-Org-Slug header. The full secret is never stored; only key_hash is.
type APIKey struct {
	bun.BaseModel `bun:"table:api_keys,alias:api_key"`

	ID         uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	OrgSlug    string     `json:"org_slug" bun:"org_slug,notnull"`
	UserID     string     `json:"user_id" bun:"user_id,notnull"`
	Name       string     `json:"name" bun:"name,notnull"`
	KeyPrefix  string     `json:"key_prefix" bun:"key_prefix,notnull"`
	KeyHash    string     `json:"-" bun:"key_hash,notnull,unique"`
	Scopes     []string   `json:"scopes" bun:"scopes,type:jsonb,notnull,default:'[]'"`
	Status     string     `json:"status" bun:"status,notnull,default:'active'"`
	LastUsedAt *time.Time `json:"last_used_at" bun:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at" bun:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at" bun:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
