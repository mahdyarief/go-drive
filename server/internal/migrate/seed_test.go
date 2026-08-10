package migrate

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	"go-drive/server/internal/config"

	_ "modernc.org/sqlite"
)

// newTestSQLite opens an in-memory SQLite DB with WAL disabled (memory DBs
// can't use WAL) but the same busy_timeout pragma the app uses.
func newTestSQLite(t *testing.T) *bun.DB {
	t.Helper()
	dsn := "file:seedtest?mode=memory&cache=shared&_pragma=busy_timeout(10000)&_pragma=foreign_keys(1)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return bun.NewDB(sqlDB, sqlitedialect.New())
}

func TestSeedDefaultLocalStore(t *testing.T) {
	ctx := context.Background()
	db := newTestSQLite(t)

	// CreateTenantTables for a fresh slug must seed a local store + primary.
	if err := CreateTenantTables(ctx, db, "fresh-org"); err != nil {
		t.Fatalf("CreateTenantTables: %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stores").Scan(&count); err != nil {
		t.Fatalf("count stores: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 seeded store, got %d", count)
	}

	var name, provider, status string
	var cfg string
	if err := db.QueryRowContext(ctx, "SELECT name, provider, status, config FROM stores").Scan(&name, &provider, &status, &cfg); err != nil {
		t.Fatalf("scan store: %v", err)
	}
	if name != "Local Storage" || provider != "local" || status != "active" {
		t.Fatalf("unexpected store row: name=%q provider=%q status=%q", name, provider, status)
	}
	if cfg == "" {
		t.Fatal("config should not be empty")
	}

	var workspaceID, primaryID string
	if err := db.QueryRowContext(ctx, "SELECT workspace_id, primary_store_id FROM workspace_storage_settings").Scan(&workspaceID, &primaryID); err != nil {
		t.Fatalf("scan setting: %v", err)
	}
	if workspaceID == "" || primaryID == "" {
		t.Fatalf("empty primary setting: workspace=%q primary=%q", workspaceID, primaryID)
	}

	// Idempotency: running again must NOT add a second store.
	if err := CreateTenantTables(ctx, db, "fresh-org"); err != nil {
		t.Fatalf("re-run CreateTenantTables: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stores").Scan(&count); err != nil {
		t.Fatalf("recount stores: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected still 1 store after re-run, got %d", count)
	}
}

func TestSeedSkipsExistingStores(t *testing.T) {
	ctx := context.Background()
	db := newTestSQLite(t)

	if err := CreateTenantTables(ctx, db, "existing-org"); err != nil {
		t.Fatalf("CreateTenantTables: %v", err)
	}
	// Insert a second store manually (simulating a tenant that attached gdrive).
	if _, err := db.ExecContext(ctx, "INSERT INTO stores (id, name, provider, status) VALUES (?, 'gdrive-1', 'gdrive', 'active')", "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("insert manual store: %v", err)
	}

	if err := CreateTenantTables(ctx, db, "existing-org"); err != nil {
		t.Fatalf("re-run CreateTenantTables: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM stores").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 stores (existing gdrive + seed skipped), got %d", count)
	}
}

func TestLocalStoreBaseDir(t *testing.T) {
	t.Setenv("SQLITE_PATH", filepath.Join(t.TempDir(), "app.db"))
	dir := config.LocalStoreBaseDir("my-org")
	if dir == "" {
		t.Fatal("LocalStoreBaseDir returned empty")
	}
	if filepath.Base(filepath.Dir(dir)) != "storage" {
		t.Fatalf("expected .../storage/<slug>, got %s", dir)
	}
	if filepath.Base(dir) != "my-org" {
		t.Fatalf("expected slug suffix, got %s", dir)
	}
	_ = os.Getenv
}
