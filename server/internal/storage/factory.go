package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
)

// New builds a Storage provider from cfg (Locker's createStorageFromConfig
// equivalent). Supported providers: local, s3, gdrive.
func New(ctx context.Context, cfg Config) (Storage, error) {
	switch cfg.Provider {
	case "local":
		dir := cfg.BaseDir
		if dir == "" {
			dir = os.Getenv("LOCAL_BLOB_DIR")
		}
		if dir == "" {
			dir = "./data/blobs"
		}
		return NewLocal(dir, deriveSignKey(), cfg.PublicURL)
	case "s3", "r2", "b2", "wasabi", "spaces", "hetzner", "idrivee2", "storj":
		return NewS3(ctx, cfg)
	case "gdrive":
		return NewGDrive(cfg.ClientID, cfg.ClientSecret, cfg.RefreshToken, cfg.FolderID)
	default:
		return nil, fmt.Errorf("storage: unsupported provider %q", cfg.Provider)
	}
}

// deriveSignKey produces the HMAC key used to sign local serve URLs. It must
// match the serve handler, so it is derived from the same env vars the crypto
// package uses (SECRETS_ENCRYPTION_KEY with AUTHULA_SECRET fallback).
func deriveSignKey() []byte {
	secret := os.Getenv("SECRETS_ENCRYPTION_KEY")
	if secret == "" {
		secret = os.Getenv("AUTHULA_SECRET")
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// SignKey returns the derived HMAC key used for local file serve URLs, so
// the serve handler can verify signatures without building a provider.
func SignKey() []byte { return deriveSignKey() }
