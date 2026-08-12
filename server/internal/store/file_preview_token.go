package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// CreatePreviewToken inserts a preview token row scoped to the tenant schema.
func CreatePreviewToken(ctx context.Context, tx bun.IDB, fileID uuid.UUID, userID, tokenHash string, expiresAt time.Time) error {
	tok := &model.FilePreviewToken{
		ID:        uuid.New(),
		FileID:    fileID,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}
	if _, err := tx.NewInsert().Model(tok).Exec(ctx); err != nil {
		return fmt.Errorf("store: creating preview token: %w", err)
	}
	return nil
}

// GetPreviewTokenByHash returns the preview token row with the given hash.
func GetPreviewTokenByHash(ctx context.Context, tx bun.IDB, tokenHash string) (*model.FilePreviewToken, error) {
	var tok model.FilePreviewToken
	if err := tx.NewSelect().Model(&tok).Where("token_hash = ?", tokenHash).Scan(ctx); err != nil {
		return nil, fmt.Errorf("store: loading preview token: %w", err)
	}
	return &tok, nil
}
