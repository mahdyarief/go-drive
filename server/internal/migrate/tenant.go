package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"

	"go-drive/server/internal/config"
)

// sqliteDDL rewrites Postgres-only DDL fragments to SQLite-compatible forms.
// gen_random_uuid() has no SQLite equivalent (the app always sets IDs in Go),
// now() becomes CURRENT_TIMESTAMP, timestamptz becomes datetime, and jsonb
// becomes json (stored as TEXT under SQLite's dynamic typing).
var sqliteDDL = strings.NewReplacer(
	"uuid PRIMARY KEY DEFAULT gen_random_uuid()", "TEXT PRIMARY KEY",
	"timestamptz", "datetime",
	"DEFAULT now()", "DEFAULT CURRENT_TIMESTAMP",
	"jsonb", "json",
)

// isSQLite reports whether the given Bun handle uses the SQLite dialect.
func isSQLite(db bun.IDB) bool {
	return db.Dialect().Name() == dialect.SQLite
}

// sqliteEnsureColumn adds a column to an existing SQLite table when it is
// missing. SQLite has no ADD COLUMN IF NOT EXISTS, so existence is checked via
// PRAGMA table_info first.
func sqliteEnsureColumn(ctx context.Context, db bun.IDB, table, column, ddl string) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, ddl))
	return err
}

// CreateTenantSchema creates the tenant schema for the given org slug.
// Postgres: creates a `tenant_<slug>` schema. SQLite has no schemas, so
// tenant tables (if any) are created in the shared database file instead.
func CreateTenantSchema(ctx context.Context, db bun.IDB, slug string) error {
	if isSQLite(db) {
		return CreateTenantTables(ctx, db, slug)
	}

	schemaName := "tenant_" + slug
	query := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pq.QuoteIdentifier(schemaName))
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("creating schema %s: %w", schemaName, err)
	}
	return CreateTenantTables(ctx, db, slug)
}

// CreateTenantTables creates tables in the tenant schema.
// Business tables are added here as the boilerplate grows.
func CreateTenantTables(ctx context.Context, db bun.IDB, slug string) error {
	schema := "tenant_" + slug

	// Postgres requires schema-qualified names; SQLite has no schemas so
	// table names are left unqualified (per-tenant file DB).
	prefix := ""
	if !isSQLite(db) {
		prefix = schema + "."
	}
	// q applies the schema prefix to the first %s verb and passes any
	// additional verbs through to fmt.Sprintf (e.g. index DDL that names
	// the same table twice).
	q := func(body string, args ...any) string {
		return fmt.Sprintf(body, append([]any{prefix}, args...)...)
	}

	queries := []string{
		// Storage stores + secrets
		q(`CREATE TABLE IF NOT EXISTS %sstores (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			name text NOT NULL,
			provider text NOT NULL,
			credential_source text NOT NULL DEFAULT 'store',
			status text NOT NULL DEFAULT 'active',
			write_mode text NOT NULL DEFAULT 'write',
			ingest_mode text NOT NULL DEFAULT 'none',
			read_priority integer NOT NULL DEFAULT 100,
			config jsonb NOT NULL DEFAULT '',
			quota_used bigint NOT NULL DEFAULT 0,
			quota_limit bigint NOT NULL DEFAULT 0,
			last_tested_at timestamptz,
			last_synced_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %sstore_secrets (
			store_id uuid PRIMARY KEY,
			encryption_version integer NOT NULL DEFAULT 1,
			encrypted_credentials text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %sworkspace_storage_settings (
			workspace_id uuid PRIMARY KEY,
			primary_store_id uuid NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),

		// File blobs + locations
		q(`CREATE TABLE IF NOT EXISTS %sfile_blobs (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			created_by_id text,
			object_key text NOT NULL,
			byte_size bigint NOT NULL,
			mime_type text NOT NULL,
			checksum text,
			state text NOT NULL DEFAULT 'pending',
			metadata jsonb,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (object_key)
		)`),
		q(`CREATE TABLE IF NOT EXISTS %sblob_locations (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			blob_id uuid NOT NULL,
			store_id uuid NOT NULL,
			storage_path text NOT NULL,
			state text NOT NULL DEFAULT 'pending',
			origin text NOT NULL,
			last_verified_at timestamptz,
			last_error text,
			metadata jsonb,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (blob_id, store_id),
			UNIQUE (store_id, storage_path)
		)`),

		// Folders + files
		q(`CREATE TABLE IF NOT EXISTS %sfolders (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id text NOT NULL,
			parent_id uuid,
			name text NOT NULL,
			color text,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %sfiles (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id text NOT NULL,
			folder_id uuid,
			blob_id uuid NOT NULL,
			name text NOT NULL,
			mime_type text NOT NULL,
			size bigint NOT NULL,
			storage_path text NOT NULL,
			storage_provider text NOT NULL,
			status text NOT NULL DEFAULT 'ready',
			thumbnail_path text,
			checksum text,
			s3_key text,
			replaces_file_id uuid,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE UNIQUE INDEX IF NOT EXISTS %sfiles_unique_name_in_folder_idx ON %sfiles (folder_id, name) WHERE status = 'ready' AND folder_id IS NOT NULL`, prefix),
		q(`CREATE UNIQUE INDEX IF NOT EXISTS %sfiles_unique_name_at_root_idx ON %sfiles (name) WHERE status = 'ready' AND folder_id IS NULL`, prefix),

		// Tags
		q(`CREATE TABLE IF NOT EXISTS %stags (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			name text NOT NULL,
			slug text NOT NULL,
			color text,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (name),
			UNIQUE (slug)
		)`),
		q(`CREATE TABLE IF NOT EXISTS %sfile_tags (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			file_id uuid NOT NULL,
			tag_id uuid NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (file_id, tag_id)
		)`),

		// Links
		q(`CREATE TABLE IF NOT EXISTS %sshare_links (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id text NOT NULL,
			file_id uuid,
			folder_id uuid,
			token text NOT NULL UNIQUE,
			access text NOT NULL DEFAULT 'download',
			has_password boolean NOT NULL DEFAULT false,
			password_hash text,
			expires_at timestamptz,
			max_downloads integer,
			download_count integer NOT NULL DEFAULT 0,
			is_active boolean NOT NULL DEFAULT true,
			last_accessed_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %supload_links (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id text NOT NULL,
			folder_id uuid,
			token text NOT NULL UNIQUE,
			name text NOT NULL,
			max_files integer,
			max_file_size bigint,
			allowed_mime_types jsonb,
			files_uploaded integer NOT NULL DEFAULT 0,
			has_password boolean NOT NULL DEFAULT false,
			password_hash text,
			expires_at timestamptz,
			is_active boolean NOT NULL DEFAULT true,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %stracked_links (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id text NOT NULL,
			file_id uuid,
			folder_id uuid,
			token text NOT NULL UNIQUE,
			name text NOT NULL,
			description text,
			access text NOT NULL DEFAULT 'view',
			has_password boolean NOT NULL DEFAULT false,
			password_hash text,
			require_email boolean NOT NULL DEFAULT false,
			expires_at timestamptz,
			valid_from timestamptz,
			valid_until timestamptz,
			max_views integer,
			view_count integer NOT NULL DEFAULT 0,
			download_count integer NOT NULL DEFAULT 0,
			is_active boolean NOT NULL DEFAULT true,
			last_accessed_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %stracked_link_events (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			tracked_link_id uuid NOT NULL,
			event_type text NOT NULL DEFAULT 'view',
			timestamp timestamptz NOT NULL DEFAULT now(),
			visitor_id text,
			email text,
			ip_address text,
			country text,
			country_code text,
			region text,
			city text,
			latitude real,
			longitude real,
			user_agent text,
			browser text,
			browser_version text,
			os text,
			os_version text,
			device_type text,
			referrer text,
			utm_source text,
			utm_medium text,
			utm_campaign text,
			language text,
			duration_seconds integer
		)`),

		// S3 gateway
		q(`CREATE TABLE IF NOT EXISTS %ss3_api_keys (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id text NOT NULL,
			access_key_id text NOT NULL UNIQUE,
			encrypted_secret text NOT NULL,
			name text NOT NULL,
			permissions text NOT NULL DEFAULT 'readwrite',
			is_active boolean NOT NULL DEFAULT true,
			last_used_at timestamptz,
			expires_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %ss3_multipart_uploads (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			upload_id text NOT NULL UNIQUE,
			s3_key text NOT NULL,
			storage_path text NOT NULL,
			content_type text NOT NULL,
			user_id text NOT NULL,
			status text NOT NULL DEFAULT 'in_progress',
			created_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %ss3_multipart_parts (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			upload_id text NOT NULL,
			part_number integer NOT NULL,
			storage_path text NOT NULL,
			size bigint NOT NULL DEFAULT 0,
			etag text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (upload_id, part_number)
		)`),

		// Replication
		q(`CREATE TABLE IF NOT EXISTS %sreplication_runs (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			kind text NOT NULL,
			status text NOT NULL DEFAULT 'queued',
			source_store_id uuid,
			target_store_id uuid,
			triggered_by_user_id text,
			total_items integer NOT NULL DEFAULT 0,
			processed_items integer NOT NULL DEFAULT 0,
			failed_items integer NOT NULL DEFAULT 0,
			error_message text,
			started_at timestamptz,
			completed_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`),
		q(`CREATE TABLE IF NOT EXISTS %sreplication_run_items (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			run_id uuid NOT NULL,
			blob_id uuid NOT NULL,
			source_store_id uuid,
			target_store_id uuid NOT NULL,
			status text NOT NULL DEFAULT 'pending',
			attempt_count integer NOT NULL DEFAULT 0,
			error_message text,
			started_at timestamptz,
			completed_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (run_id, blob_id, target_store_id)
		)`),
		q(`CREATE TABLE IF NOT EXISTS %singest_tombstones (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			store_id uuid NOT NULL,
			external_path text NOT NULL,
			deleted_blob_id uuid,
			deleted_by_user_id text,
			reason text NOT NULL DEFAULT 'user_deleted',
			created_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (store_id, external_path)
		)`),

		// Transcription (search fallback)
		q(`CREATE TABLE IF NOT EXISTS %sfile_transcriptions (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			file_id uuid NOT NULL,
			plugin_slug text NOT NULL,
			content text NOT NULL DEFAULT '',
			status text NOT NULL DEFAULT 'pending',
			error_message text,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (file_id, plugin_slug)
		)`),
	}

	// Postgres additive alterations ride along in the same loop (ADD COLUMN
	// IF NOT EXISTS is a no-op on fresh schemas). SQLite's run below after
	// the loop because ALTER TABLE fails when the table does not exist yet
	// (fresh tenant), and sqliteEnsureColumn's PRAGMA guard needs the table
	// present to detect already-added columns.
	if !isSQLite(db) {
		queries = append(queries,
			q(`ALTER TABLE %sstores ADD COLUMN IF NOT EXISTS quota_used bigint NOT NULL DEFAULT 0`),
			q(`ALTER TABLE %sstores ADD COLUMN IF NOT EXISTS quota_limit bigint NOT NULL DEFAULT 0`),
		)
	}

	for _, q := range queries {
		if isSQLite(db) {
			q = sqliteDDL.Replace(q)
		}
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("creating tenant table in %s: %w", schema, err)
		}
	}

	if isSQLite(db) {
		if err := sqliteEnsureColumn(ctx, db, "stores", "quota_used", "quota_used bigint NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("adding column to %s.stores: %w", schema, err)
		}
		if err := sqliteEnsureColumn(ctx, db, "stores", "quota_limit", "quota_limit bigint NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("adding column to %s.stores: %w", schema, err)
		}
	}

	// Seed a ready-to-use local store so a fresh workspace can store files
	// without attaching an external provider first. No-op when the tenant
	// already has stores (existing tenants keep their configuration).
	if err := seedDefaultLocalStore(ctx, db, prefix, slug); err != nil {
		return err
	}

	return nil
}

// seedDefaultLocalStore inserts a local provider store as the workspace
// primary when the stores table is empty. Runs inside CreateTenantTables so
// both new tenants and backfilled tenants (RunTenantMigrations on Postgres,
// first DB open on SQLite) get a working default.
func seedDefaultLocalStore(ctx context.Context, db bun.IDB, prefix, slug string) error {
	var count int
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %sstores", prefix)).Scan(&count); err != nil {
		return fmt.Errorf("seeding local store: counting stores: %w", err)
	}
	if count > 0 {
		return nil
	}

	storeID := uuid.New()
	cfg, err := json.Marshal(map[string]any{
		"baseDir":   config.LocalStoreBaseDir(slug),
		"publicUrl": config.BaseURL(),
	})
	if err != nil {
		return fmt.Errorf("seeding local store: marshaling config: %w", err)
	}
	cfgLit := string(cfg)

	insert := fmt.Sprintf("INSERT INTO %sstores (id, name, provider, status, write_mode, config) VALUES (?, 'Local Storage', 'local', 'active', 'write', ?)", prefix)
	setting := fmt.Sprintf("INSERT INTO %sworkspace_storage_settings (workspace_id, primary_store_id) VALUES (?, ?)", prefix)
	if !isSQLite(db) {
		insert = fmt.Sprintf("INSERT INTO %sstores (id, name, provider, status, write_mode, config) VALUES ($1, 'Local Storage', 'local', 'active', 'write', $2::jsonb)", prefix)
		setting = fmt.Sprintf("INSERT INTO %sworkspace_storage_settings (workspace_id, primary_store_id) VALUES ($1, $2)", prefix)
	}
	if _, err := db.ExecContext(ctx, insert, storeID.String(), cfgLit); err != nil {
		return fmt.Errorf("seeding local store: %w", err)
	}
	if _, err := db.ExecContext(ctx, setting, uuid.Nil.String(), storeID.String()); err != nil {
		return fmt.Errorf("seeding primary store: %w", err)
	}
	return nil
}

// RunTenantMigrations runs tenant migrations for all existing org schemas.
// SQLite has no schemas, so there is nothing to enumerate.
func RunTenantMigrations(ctx context.Context, db *bun.DB) error {
	if isSQLite(db) {
		return nil
	}

	rows, err := db.QueryContext(ctx, `SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'tenant_%'`)
	if err != nil {
		return fmt.Errorf("listing tenant schemas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return fmt.Errorf("scanning schema name: %w", err)
		}
		slug := schemaName[len("tenant_"):]
		if err := CreateTenantTables(ctx, db, slug); err != nil {
			return fmt.Errorf("migrating schema %s: %w", schemaName, err)
		}
	}
	return rows.Err()
}

// DropTenantSchema drops the tenant schema for the given org slug.
// SQLite has no schemas, so this is a no-op (tables live in one file).
func DropTenantSchema(ctx context.Context, db bun.IDB, slug string) error {
	if isSQLite(db) {
		return nil
	}

	schemaName := "tenant_" + slug
	query := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", pq.QuoteIdentifier(schemaName))
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("dropping schema %s: %w", schemaName, err)
	}
	return nil
}
