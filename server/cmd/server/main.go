package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/uptrace/bun"

	"go-drive/server/internal/config"
	"go-drive/server/internal/migrate"
	"go-drive/server/internal/router"
	"go-drive/server/internal/store"
	"go-drive/server/internal/tenant"
)

//go:embed static/*
var embeddedFiles embed.FS

func main() {
	_ = godotenv.Load() // Load .env if present (no error if missing)

	db := config.NewDB()
	auth := config.NewAuth(db)

	if err := migrate.RunPublicMigrations(context.Background(), db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	if err := migrate.RunTenantMigrations(context.Background(), db); err != nil {
		log.Fatalf("Failed to run tenant migrations: %v", err)
	}

	// Start background tiering job
	go runBackgroundTiering(db)

	// Serve embedded frontend if static/ was populated at build time
	var staticFiles fs.FS
	if entries, _ := fs.ReadDir(embeddedFiles, "static"); len(entries) > 0 {
		staticFiles, _ = fs.Sub(embeddedFiles, "static")
	}

	r := router.New(auth, db, staticFiles)

	log.Printf("Server starting on %s", config.Port())
	if err := r.Run(config.Port()); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// runBackgroundTiering runs storage tiering every 24 hours for all tenants.
func runBackgroundTiering(db *bun.DB) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run once at startup
	runTieringForAllTenants(db)

	for range ticker.C {
		runTieringForAllTenants(db)
	}
}

// runTieringForAllTenants iterates through all orgs and runs tiering.
func runTieringForAllTenants(db *bun.DB) {
	ctx := context.Background()

	// Get all org slugs from the public schema
	var orgs []struct {
		Slug string `bun:"slug"`
	}
	if err := db.NewSelect().TableExpr("orgs").Column("slug").Scan(ctx, &orgs); err != nil {
		log.Printf("tiering: listing orgs: %v", err)
		return
	}

	for _, org := range orgs {
		tx, err := tenant.OpenTx(ctx, db, org.Slug)
		if err != nil {
			log.Printf("tiering: opening tx for %s: %v", org.Slug, err)
			continue
		}

		policy, err := store.GetTieringPolicy(ctx, tx)
		if err != nil {
			tx.Rollback()
			continue
		}

		if !policy.Enabled {
			tx.Rollback()
			continue
		}

		count, err := store.RunStorageTiering(ctx, tx, policy)
		if err != nil {
			log.Printf("tiering: running for %s: %v", org.Slug, err)
			tx.Rollback()
			continue
		}

		if err := tx.Commit(); err != nil {
			log.Printf("tiering: committing for %s: %v", org.Slug, err)
			continue
		}

		if count > 0 {
			log.Printf("tiering: tiered %d files for %s", count, org.Slug)
		}
	}
}
