package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// FileTranscription holds AI-transcribed text for a file (search fallback).
type FileTranscription struct {
	bun.BaseModel `bun:"table:file_transcriptions"`

	ID           uuid.UUID `json:"id" bun:"id,pk,type:uuid"`
	FileID       uuid.UUID `json:"file_id" bun:"file_id,type:uuid,notnull"`
	PluginSlug   string    `json:"plugin_slug" bun:"plugin_slug,notnull"`
	Content      string    `json:"content" bun:"content,notnull,default:''"`
	Status       string    `json:"status" bun:"status,notnull,default:'pending'"`
	ErrorMessage string    `json:"error_message" bun:"error_message"`
	CreatedAt    time.Time `json:"created_at" bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt    time.Time `json:"updated_at" bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}
