package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"go-drive/server/internal/crypto"
	"go-drive/server/internal/model"
	"go-drive/server/internal/storage"
)

// BuildStorage hydrates a storage.Storage from a stores row, decrypting the
// per-store credentials from store_secrets. This is the Go equivalent of
// Locker's createStorageFromConfig.
func BuildStorage(ctx context.Context, tx bun.IDB, s *model.Store) (storage.Storage, error) {
	cfg := storage.Config{
		Provider:  s.Provider,
		BaseDir:   str(s.Config["baseDir"]),
		PublicURL: str(s.Config["publicUrl"]),
		Bucket:    str(s.Config["bucket"]),
		Region:    str(s.Config["region"]),
		Endpoint:  str(s.Config["endpoint"]),
		FolderID:  str(s.Config["folderId"]),
	}

	// Decrypt credentials (access keys for s3, OAuth tokens for gdrive).
	var secret model.StoreSecret
	if err := tx.NewSelect().Model(&secret).Where("store_id = ?", s.ID).Scan(ctx); err == nil {
		creds, err := crypto.Decrypt(secret.EncryptedCredentials)
		if err == nil {
			var m map[string]any
			if json.Unmarshal([]byte(creds), &m) == nil {
				cfg.AccessKey = str(m["accessKeyId"])
				cfg.SecretKey = str(m["secretAccessKey"])
				cfg.ClientID = str(m["clientId"])
				cfg.ClientSecret = str(m["clientSecret"])
				cfg.RefreshToken = str(m["refreshToken"])
			}
		}
	}

	return storage.New(ctx, cfg)
}

// FolderPath walks the parent chain and returns the display path of the
// folder (e.g. "docs/reports"). A cycle guard caps the walk at 50 levels.
func FolderPath(ctx context.Context, tx bun.IDB, id uuid.UUID) (string, error) {
	var parts []string
	cur := &id
	for cur != nil {
		var f model.Folder
		if err := tx.NewSelect().Model(&f).Where("id = ?", *cur).Scan(ctx); err != nil {
			return "", fmt.Errorf("store: loading folder %s: %w", *cur, err)
		}
		parts = append([]string{f.Name}, parts...)
		cur = f.ParentID
		if len(parts) > 50 {
			return "", fmt.Errorf("store: folder chain too deep or cyclic")
		}
	}
	return strings.Join(parts, "/"), nil
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
