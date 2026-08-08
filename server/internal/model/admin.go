package model

import (
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Admin represents an admin user. Simple join table to avoid touching Authula tables.
type Admin struct {
	bun.BaseModel `bun:"table:admins,alias:admin"`

	UserID uuid.UUID `json:"user_id" bun:"user_id,pk,type:uuid"`
}
