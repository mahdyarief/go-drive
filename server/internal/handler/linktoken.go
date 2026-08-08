package handler

import (
	"context"
	"database/sql"
	"errors"

	"github.com/uptrace/bun"
)

// linkTokenRow is a row from the public link_tokens registry table.
type linkTokenRow struct {
	Token    string `bun:"token"`
	OrgSlug  string `bun:"org_slug"`
	LinkType string `bun:"link_type"`
	LinkID   string `bun:"link_id"`
}

// resolveLinkToken looks up a public link token in the registry. It returns
// (nil, nil) when the token is unknown.
func resolveLinkToken(ctx context.Context, db *bun.DB, token string) (*linkTokenRow, error) {
	var row linkTokenRow
	err := db.QueryRowContext(ctx,
		"SELECT token, org_slug, link_type, link_id FROM link_tokens WHERE token = ?", token).
		Scan(&row.Token, &row.OrgSlug, &row.LinkType, &row.LinkID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// registerLinkToken records a public link token → tenant mapping. Writes go
// to the main DB (public schema on Postgres, main DB on SQLite) so they work
// regardless of the tenant transaction's search_path.
func registerLinkToken(ctx context.Context, db *bun.DB, token, orgSlug, linkType, linkID string) error {
	_, err := db.ExecContext(ctx,
		"INSERT INTO link_tokens (token, org_slug, link_type, link_id) VALUES (?, ?, ?, ?) ON CONFLICT (token) DO NOTHING",
		token, orgSlug, linkType, linkID)
	return err
}

// deleteLinkToken removes a public link token from the registry.
func deleteLinkToken(ctx context.Context, db *bun.DB, token string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM link_tokens WHERE token = ?", token)
	return err
}
