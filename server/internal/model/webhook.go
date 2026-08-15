package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Webhook is a tenant-level subscription to file events. When an event
// matches the webhook's event types, an HTTP POST is sent to TargetURL.
type Webhook struct {
	bun.BaseModel `bun:"table:webhooks"`

	ID          uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	UserID      string    `json:"user_id" bun:"user_id,notnull"`
	Name        string    `json:"name" bun:"name,notnull"`
	TargetURL   string    `json:"target_url" bun:"target_url,notnull"`
	Secret      string    `json:"-" bun:"secret"` // HMAC secret for signature verification
	EventTypes  []string  `json:"event_types" bun:"event_types,type:jsonb,notnull"`
	IsActive    bool      `json:"is_active" bun:"is_active,notnull,default:true"`
	LastTriggeredAt *time.Time `json:"last_triggered_at" bun:"last_triggered_at"`
	CreatedAt   time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt   time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// WebhookDelivery tracks a single delivery attempt for a webhook event.
type WebhookDelivery struct {
	bun.BaseModel `bun:"table:webhook_deliveries"`

	ID            uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	WebhookID     uuid.UUID `json:"webhook_id" bun:"webhook_id,type:uuid,notnull"`
	EventType     string    `json:"event_type" bun:"event_type,notnull"`
	Payload       string    `json:"payload" bun:"payload,notnull"`
	ResponseCode  int       `json:"response_code" bun:"response_code"`
	ResponseBody  string    `json:"response_body" bun:"response_body"`
	Success       bool      `json:"success" bun:"success,notnull,default:false"`
	Attempts      int       `json:"attempts" bun:"attempts,notnull,default:1"`
	LastAttemptAt time.Time `json:"last_attempt_at" bun:"last_attempt_at,nullzero,notnull,default:current_timestamp"`
	CreatedAt     time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
}
