package config

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"

	authula "github.com/Authula/authula"
	authulaconfig "github.com/Authula/authula/config"
	authulamodels "github.com/Authula/authula/models"
	emailplugin "github.com/Authula/authula/plugins/email"
	emailpasswordplugin "github.com/Authula/authula/plugins/email-password"
	emailpasswordtypes "github.com/Authula/authula/plugins/email-password/types"
	emailtypes "github.com/Authula/authula/plugins/email/types"
	sessionplugin "github.com/Authula/authula/plugins/session"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

const (
	DefaultPort    = ":8081"
	DefaultBaseURL = "http://localhost:8081"

	// Database drivers
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"

	DefaultSQLitePath = "./data/app.db"
)

// DBDriver returns the configured database driver: "postgres" (default) or "sqlite".
func DBDriver() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DB_DRIVER"))) {
	case DriverSQLite:
		return DriverSQLite
	default:
		return DriverPostgres
	}
}

// IsSQLite reports whether DB_DRIVER=sqlite is configured.
func IsSQLite() bool { return DBDriver() == DriverSQLite }

// SQLitePath returns the SQLite database file path from SQLITE_PATH or the default.
func SQLitePath() string {
	if p := os.Getenv("SQLITE_PATH"); p != "" {
		return p
	}
	return DefaultSQLitePath
}

// DatabaseURL returns the connection string for the configured driver
// (Postgres DSN or SQLite file path) — used for Authula's DatabaseConfig.
func DatabaseURL() string {
	if IsSQLite() {
		return SQLitePath()
	}
	return os.Getenv("DATABASE_URL")
}

// SQLiteTenantDir returns the directory holding per-tenant SQLite files
// (SQLite mode only — the counterpart of Postgres tenant schemas).
func SQLiteTenantDir() string {
	return filepath.Join(filepath.Dir(SQLitePath()), "tenants")
}

// SQLiteTenantPath returns the database file path for a tenant slug.
// Mirrors the `tenant_<slug>` schema naming used on Postgres.
func SQLiteTenantPath(slug string) string {
	return filepath.Join(SQLiteTenantDir(), "tenant_"+slug+".db")
}

// LocalStoreBaseDir returns the default base directory for a tenant's
// seeded local store, e.g. ./data/storage/<slug>. Derived from the SQLite
// path so both drivers keep blob storage next to the app data directory.
func LocalStoreBaseDir(slug string) string {
	return filepath.Join(filepath.Dir(SQLitePath()), "storage", slug)
}

// SQLiteMaxOpenConns returns the max open connections for SQLite pools from
// SQLITE_MAX_OPEN_CONNS or the default (8). A pool of 8 allows concurrent reads
// and parallel writes with busy_timeout absorbing contention.
func SQLiteMaxOpenConns() int {
	if v := os.Getenv("SQLITE_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 8
}

// NewDB creates a Bun database connection to the configured driver.
func NewDB() *bun.DB {
	if IsSQLite() {
		return newSQLiteDB()
	}
	return newPostgresDB()
}

func newPostgresDB() *bun.DB {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL environment variable is required (or set DB_DRIVER=sqlite)")
	}

	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(10 * time.Minute)

	return bun.NewDB(sqlDB, pgdialect.New())
}

func newSQLiteDB() *bun.DB {
	path := SQLitePath()
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("failed to create sqlite directory: %v", err)
		}
	}

	// WAL + busy timeout + foreign keys + synchronous(NORMAL) via modernc
	// pragma DSN params. NORMAL is the recommended durability setting for WAL:
	// commits skip fsync (only checkpoint syncs), giving much faster writes.
	// 60s busy_timeout handles concurrent write contention on Fly volumes.
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(60000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// WAL allows concurrent readers + one writer. A modest pool parallelizes
	// reads for mid-to-high traffic; busy_timeout absorbs write contention.
	pool := SQLiteMaxOpenConns()
	sqlDB.SetMaxOpenConns(pool)
	sqlDB.SetMaxIdleConns(pool)

	return bun.NewDB(sqlDB, sqlitedialect.New())
}

// NewAuth creates and configures an Authula instance with email/password
// and session plugins. Uses the shared connection for storage.
func NewAuth(db bun.IDB) *authula.Auth {
	cfg := authulaconfig.NewConfig(
		authulaconfig.WithAppName("GinReactMonorepo"),
		authulaconfig.WithBaseURL(BaseURL()),
		authulaconfig.WithBasePath("/auth"),
		authulaconfig.WithSecret(os.Getenv("AUTHULA_SECRET")),
		authulaconfig.WithDatabase(authulamodels.DatabaseConfig{
			Provider: DBDriver(),
			URL:      DatabaseURL(),
		}),
	)

	return authula.New(&authula.AuthConfig{
		Config: cfg,
		DB:     db,
		Plugins: []authulamodels.Plugin{
			emailplugin.New(emailtypes.EmailPluginConfig{
				Enabled:     true,
				Provider:    emailtypes.ProviderSMTP,
				FromAddress: "noreply@localhost",
				SMTP: &emailtypes.SMTPConfig{
					Host: "localhost",
					Port: 1025,
				},
			}),
			emailpasswordplugin.New(emailpasswordtypes.EmailPasswordPluginConfig{
				Enabled:                  true,
				MinPasswordLength:        8,
				MaxPasswordLength:        32,
				RequireEmailVerification: false,
				AutoSignIn:               true,
			}),
			sessionplugin.New(sessionplugin.SessionPluginConfig{
				Enabled: true,
			}),
		},
	})
}

// Port returns the server port from PORT env var or the default.
func Port() string {
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return DefaultPort
}

// BaseURL returns the server base URL from AUTHULA_BASE_URL env var or the default.
func BaseURL() string {
	if url := os.Getenv("AUTHULA_BASE_URL"); url != "" {
		return url
	}
	return DefaultBaseURL
}
