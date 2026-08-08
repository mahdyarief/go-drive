package tenant

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	"go-drive/server/internal/config"
	"go-drive/server/internal/migrate"

	_ "modernc.org/sqlite"
)

var (
	mu  sync.Mutex
	dbs = map[string]*bun.DB{}
)

// DB returns the SQLite database for a tenant, opening + migrating it on
// first use. Only used in SQLite mode (file-per-tenant isolation).
func DB(ctx context.Context, slug string) (*bun.DB, error) {
	mu.Lock()
	defer mu.Unlock()
	if db, ok := dbs[slug]; ok {
		return db, nil
	}

	path := config.SQLiteTenantPath(slug)
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	// Same pragmas as the main DB: WAL + busy timeout + foreign keys +
	// synchronous(NORMAL).
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	pool := config.SQLiteMaxOpenConns()
	sqlDB.SetMaxOpenConns(pool)
	sqlDB.SetMaxIdleConns(pool)

	db := bun.NewDB(sqlDB, sqlitedialect.New())
	if err := migrate.CreateTenantTables(ctx, db, slug); err != nil {
		db.Close()
		return nil, err
	}

	dbs[slug] = db
	return db, nil
}

// Drop closes the cached connection and removes the tenant's SQLite file.
// Only used in SQLite mode.
func Drop(slug string) error {
	mu.Lock()
	defer mu.Unlock()
	if db, ok := dbs[slug]; ok {
		db.Close()
		delete(dbs, slug)
	}
	if err := os.Remove(config.SQLiteTenantPath(slug)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
