package model

import "github.com/uptrace/bun"

// AppSetting is a global key-value setting in the public schema.
// Used for UI-managed app configuration (e.g. Google Drive credentials).
type AppSetting struct {
	bun.BaseModel `bun:"table:app_settings,alias:app_setting"`

	Key   string `json:"key" bun:"key,pk"`
	Value string `json:"value" bun:"value,notnull,default:''"`
}
