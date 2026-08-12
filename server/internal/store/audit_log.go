package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// CreateAuditLog inserts a best-effort audit entry. Callers log-and-continue
// on error so a failed audit never fails the main operation.
func CreateAuditLog(ctx context.Context, tx bun.IDB, userID string, action, entityType, entityID string, metadata map[string]any) error {
	log := &model.AuditLog{
		ID:         uuid.New(),
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Metadata:   metadata,
	}
	if _, err := tx.NewInsert().Model(log).Exec(ctx); err != nil {
		return fmt.Errorf("store: creating audit log: %w", err)
	}
	return nil
}

// ListAuditLogs returns the user's most recent audit entries, newest first.
func ListAuditLogs(ctx context.Context, tx bun.IDB, userID string, limit int) ([]model.AuditLog, error) {
	var logs []model.AuditLog
	if err := tx.NewSelect().Model(&logs).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("store: listing audit logs: %w", err)
	}
	return logs, nil
}
