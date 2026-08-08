package tenant

import (
	"context"
	"fmt"

	"github.com/lib/pq"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

// OpenTx begins a transaction scoped to the tenant schema for orgSlug,
// without requiring a user membership check. Used by public link endpoints
// (share/upload/tracked) that resolve the owning org from a link token
// instead of an X-Org-Slug header. The caller must commit or roll back.
func OpenTx(ctx context.Context, db *bun.DB, orgSlug string) (bun.Tx, error) {
	if db.Dialect().Name() == dialect.SQLite {
		tdb, err := DB(ctx, orgSlug)
		if err != nil {
			return bun.Tx{}, err
		}
		return tdb.BeginTx(ctx, nil)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return bun.Tx{}, err
	}
	schemaName := "tenant_" + orgSlug
	query := fmt.Sprintf("SET LOCAL search_path TO %s, public", pq.QuoteIdentifier(schemaName))
	if _, err := tx.ExecContext(ctx, query); err != nil {
		tx.Rollback()
		return bun.Tx{}, err
	}
	return tx, nil
}
