package migrate

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"go-drive/server/internal/model"
)

// RunPublicMigrations creates the shared tables in the public schema.
func RunPublicMigrations(ctx context.Context, db *bun.DB) error {
	// Organizations
	if _, err := db.NewCreateTable().
		Model((*model.Organization)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("creating organizations table: %w", err)
	}

	// Organization members
	if _, err := db.NewCreateTable().
		Model((*model.OrganizationMember)(nil)).
		IfNotExists().
		ForeignKey(`("organization_id") REFERENCES "organizations" ("id") ON DELETE CASCADE`).
		Exec(ctx); err != nil {
		return fmt.Errorf("creating organization_members table: %w", err)
	}

	if _, err := db.NewCreateIndex().
		Model((*model.OrganizationMember)(nil)).
		Index("idx_org_members_org_user").
		Column("organization_id", "user_id").
		Unique().
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("creating unique index on organization_members: %w", err)
	}

	// Admin users
	if _, err := db.NewCreateTable().
		Model((*model.Admin)(nil)).
		IfNotExists().
		ForeignKey(`("user_id") REFERENCES "users" ("id") ON DELETE CASCADE`).
		Exec(ctx); err != nil {
		return fmt.Errorf("creating admins table: %w", err)
	}

	// Per-user storage limits — admin assigns a quota to each user
	// (0 = unlimited); owners then slice it across their orgs via org_quotas.
	if _, err := db.NewCreateTable().
		Model((*model.UserQuota)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return fmt.Errorf("creating user_quotas table: %w", err)
	}

	// Per-org storage allocation — the slice of the owner's quota assigned
	// to this org. Dropped automatically when the org is deleted so the
	// owner's quota is freed again.
	if _, err := db.NewCreateTable().
		Model((*model.OrgQuota)(nil)).
		IfNotExists().
		ForeignKey(`("organization_id") REFERENCES "organizations" ("id") ON DELETE CASCADE`).
		Exec(ctx); err != nil {
		return fmt.Errorf("creating org_quotas table: %w", err)
	}

	// Global app settings (key-value) — UI-managed config (e.g. register-disabled).
	// Unqualified name: resolves to the `public` schema on Postgres, the main
	// database on SQLite.
	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS app_settings (
			key text PRIMARY KEY,
			value text NOT NULL DEFAULT ''
		)
	`).Exec(ctx); err != nil {
		return fmt.Errorf("creating app_settings table: %w", err)
	}

	// Public link registry — maps a public link token to the tenant schema
	// that owns it, so public endpoints (/api/shared/:token etc.) can resolve
	// which tenant to open without requiring an X-Org-Slug header.
	linkTokensDDL := `
		CREATE TABLE IF NOT EXISTS link_tokens (
			token text PRIMARY KEY,
			org_slug text NOT NULL,
			link_type text NOT NULL,
			link_id text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_link_tokens_link ON link_tokens (link_type, link_id);
	`
	if isSQLite(db) {
		linkTokensDDL = sqliteDDL.Replace(linkTokensDDL)
	}
	if _, err := db.ExecContext(ctx, linkTokensDDL); err != nil {
		return fmt.Errorf("creating link_tokens table: %w", err)
	}

	// External API keys — cross-tenant credentials for the public upload API.
	// Stored in the PUBLIC schema (not tenant schemas) so the API-key
	// middleware can resolve the owning org from the key hash BEFORE opening
	// any tenant transaction. The full secret is never stored; only its
	// SHA-256 hash (key_hash) is persisted.
	apiKeysDDL := `
		CREATE TABLE IF NOT EXISTS api_keys (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			org_slug text NOT NULL,
			user_id text NOT NULL,
			name text NOT NULL,
			key_prefix text NOT NULL,
			key_hash text NOT NULL UNIQUE,
			scopes jsonb NOT NULL DEFAULT '[]',
			status text NOT NULL DEFAULT 'active',
			last_used_at timestamptz,
			expires_at timestamptz,
			revoked_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_api_keys_org ON api_keys (org_slug);
	`
	if isSQLite(db) {
		apiKeysDDL = sqliteDDL.Replace(apiKeysDDL)
	}
	if _, err := db.ExecContext(ctx, apiKeysDDL); err != nil {
		return fmt.Errorf("creating api_keys table: %w", err)
	}

	return nil
}
