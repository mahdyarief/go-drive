package store

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// GetTieringPolicy loads the tenant's tiering policy. Returns a default
// disabled policy if no row exists.
func GetTieringPolicy(ctx context.Context, tx bun.IDB) (*model.TieringPolicy, error) {
	var p model.TieringPolicy
	err := tx.NewSelect().Model(&p).Limit(1).Scan(ctx)
	if err != nil {
		// No policy row yet — return default.
		return &model.TieringPolicy{
			Enabled:           false,
			TierDownAfterDays: 90,
			TierUpOnAccess:    true,
			DefaultTier:       "standard",
		}, nil
	}
	return &p, nil
}

// SaveTieringPolicy upserts the tenant's tiering policy.
func SaveTieringPolicy(ctx context.Context, tx bun.IDB, p *model.TieringPolicy) error {
	p.UpdatedAt = time.Now()
	_, err := tx.NewInsert().Model(p).
		On("CONFLICT (id) DO UPDATE SET enabled = EXCLUDED.enabled, tier_down_after_days = EXCLUDED.tier_down_after_days, tier_up_on_access = EXCLUDED.tier_up_on_access, default_tier = EXCLUDED.default_tier, updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// RunStorageTiering scans for files that haven't been accessed in
// tierDownAfterDays and moves them to a cheaper tier. Returns the number
// of files tiered down.
func RunStorageTiering(ctx context.Context, tx bun.Tx, policy *model.TieringPolicy) (int, error) {
	if !policy.Enabled || policy.TierDownAfterDays <= 0 {
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -policy.TierDownAfterDays)

	// Find files in 'standard' tier that are older than cutoff and haven't
	// been accessed recently.
	var files []model.File
	err := tx.NewSelect().Model(&files).
		Where("storage_tier = 'standard'").
		Where("created_at < ?", cutoff).
		Where("(last_accessed_at IS NULL OR last_accessed_at < ?)", cutoff).
		Limit(1000).
		Scan(ctx)
	if err != nil {
		return 0, fmt.Errorf("tiering: querying cold files: %w", err)
	}

	tiered := 0
	for _, f := range files {
		newTier := "infrequent"
		_, err := tx.NewUpdate().Model((*model.File)(nil)).
			Set("storage_tier = ?", newTier).
			Set("updated_at = ?", time.Now()).
			Where("id = ?", f.ID).
			Exec(ctx)
		if err != nil {
			log.Printf("tiering: moving file %s: %v", f.ID, err)
			continue
		}
		tiered++
	}

	return tiered, nil
}

// TierUpOnAccess moves a file back to 'standard' tier when accessed, if
// the policy enables it.
func TierUpOnAccess(ctx context.Context, tx bun.IDB, fileID uuid.UUID, policy *model.TieringPolicy) error {
	if !policy.TierUpOnAccess {
		return nil
	}
	_, err := tx.NewUpdate().Model((*model.File)(nil)).
		Set("storage_tier = 'standard'").
		Set("last_accessed_at = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Where("id = ? AND storage_tier != 'standard'", fileID).
		Exec(ctx)
	return err
}

// UpdateLastAccessedAt records a file access timestamp.
func UpdateLastAccessedAt(ctx context.Context, tx bun.IDB, fileID uuid.UUID) error {
	now := time.Now()
	_, err := tx.NewUpdate().Model((*model.File)(nil)).
		Set("last_accessed_at = ?", now).
		Where("id = ?", fileID).
		Exec(ctx)
	return err
}
