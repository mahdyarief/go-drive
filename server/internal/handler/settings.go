package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// setAppSetting upserts a single key-value app setting.
func setAppSetting(ctx context.Context, db *bun.DB, key, value string) error {
	_, err := db.NewInsert().
		Model(&model.AppSetting{Key: key, Value: value}).
		On("CONFLICT (key) DO UPDATE SET value = EXCLUDED.value").
		Exec(ctx)
	return err
}

// randomState returns a cryptographically random hex string for OAuth state.
func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand never fails on supported platforms; fall back to time+pid.
		return fmt.Sprintf("%d%d", time.Now().UnixNano(), os.Getpid())
	}
	return hex.EncodeToString(b)
}
