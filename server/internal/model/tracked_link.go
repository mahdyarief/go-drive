package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// TrackedLink is an analytics-enabled public link to a file or folder.
type TrackedLink struct {
	bun.BaseModel `bun:"table:tracked_links"`

	ID             uuid.UUID  `json:"id" bun:"id,pk,type:uuid"`
	UserID         string     `json:"user_id" bun:"user_id,notnull"`
	FileID         *uuid.UUID `json:"file_id" bun:"file_id,type:uuid"`
	FolderID       *uuid.UUID `json:"folder_id" bun:"folder_id,type:uuid"`
	Token          string     `json:"token" bun:"token,notnull,unique"`
	Name           string     `json:"name" bun:"name,notnull"`
	Description    string     `json:"description" bun:"description"`
	Access         string     `json:"access" bun:"access,notnull,default:'view'"`
	HasPassword    bool       `json:"has_password" bun:"has_password,notnull,default:false"`
	PasswordHash   string     `json:"password_hash" bun:"password_hash"`
	RequireEmail   bool       `json:"require_email" bun:"require_email,notnull,default:false"`
	ExpiresAt      *time.Time `json:"expires_at" bun:"expires_at"`
	ValidFrom      *time.Time `json:"valid_from" bun:"valid_from"`
	ValidUntil     *time.Time `json:"valid_until" bun:"valid_until"`
	MaxViews       *int       `json:"max_views" bun:"max_views"`
	ViewCount      int        `json:"view_count" bun:"view_count,notnull,default:0"`
	DownloadCount  int        `json:"download_count" bun:"download_count,notnull,default:0"`
	IsActive       bool       `json:"is_active" bun:"is_active,notnull,default:true"`
	LastAccessedAt *time.Time `json:"last_accessed_at" bun:"last_accessed_at"`
	CreatedAt      time.Time  `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt      time.Time  `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// TrackedLinkEvent records a view/download on a tracked link with analytics.
type TrackedLinkEvent struct {
	bun.BaseModel `bun:"table:tracked_link_events"`

	ID              uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	TrackedLinkID   uuid.UUID `json:"tracked_link_id" bun:"tracked_link_id,type:uuid,notnull"`
	EventType       string    `json:"event_type" bun:"event_type,notnull,default:'view'"`
	Timestamp       time.Time `json:"timestamp" bun:"timestamp,nullzero,notnull,default:current_timestamp"`
	VisitorID       string    `json:"visitor_id" bun:"visitor_id"`
	Email           string    `json:"email" bun:"email"`
	IPAddress       string    `json:"ip_address" bun:"ip_address"`
	Country         string    `json:"country" bun:"country"`
	CountryCode     string    `json:"country_code" bun:"country_code"`
	Region          string    `json:"region" bun:"region"`
	City            string    `json:"city" bun:"city"`
	Latitude        *float32  `json:"latitude" bun:"latitude"`
	Longitude       *float32  `json:"longitude" bun:"longitude"`
	UserAgent       string    `json:"user_agent" bun:"user_agent"`
	Browser         string    `json:"browser" bun:"browser"`
	BrowserVersion  string    `json:"browser_version" bun:"browser_version"`
	OS              string    `json:"os" bun:"os"`
	OSVersion       string    `json:"os_version" bun:"os_version"`
	DeviceType      string    `json:"device_type" bun:"device_type"`
	Referrer        string    `json:"referrer" bun:"referrer"`
	UTMSource       string    `json:"utm_source" bun:"utm_source"`
	UTMMedium       string    `json:"utm_medium" bun:"utm_medium"`
	UTMCampaign     string    `json:"utm_campaign" bun:"utm_campaign"`
	Language        string    `json:"language" bun:"language"`
	DurationSeconds *int      `json:"duration_seconds" bun:"duration_seconds"`
}
