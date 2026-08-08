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

	return nil
}
