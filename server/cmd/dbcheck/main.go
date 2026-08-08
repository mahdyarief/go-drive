package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Println("DATABASE_URL not set")
		os.Exit(1)
	}
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		fmt.Println("ERR open:", err)
		os.Exit(1)
	}
	defer sqlDB.Close()
	db := bun.NewDB(sqlDB, pgdialect.New())
	defer db.Close()

	// List tenant schemas
	var schemas []string
	err = db.NewRaw(`SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'tenant%' ORDER BY 1`).Scan(ctx, &schemas)
	if err != nil {
		fmt.Println("ERR schemas:", err)
		os.Exit(1)
	}
	fmt.Println("TENANT SCHEMAS:", strings.Join(schemas, ", "))

	for _, s := range schemas {
		fmt.Printf("\n=== schema %s ===\n", s)
		// order_payments table exists?
		var hasTbl int
		err = db.NewRaw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = 'order_payments'`, s).Scan(ctx, &hasTbl)
		if err != nil {
			fmt.Println("  ERR table check:", err)
			continue
		}
		fmt.Printf("  order_payments table: %v\n", hasTbl > 0)

		// orders.paid_amount column?
		var hasCol int
		err = db.NewRaw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = ? AND table_name = 'orders' AND column_name = 'paid_amount'`, s).Scan(ctx, &hasCol)
		if err != nil {
			fmt.Println("  ERR col check:", err)
			continue
		}
		fmt.Printf("  orders.paid_amount column: %v\n", hasCol > 0)

		// status distribution
		type row struct {
			Status string `bun:"status"`
			N      int    `bun:"n"`
		}
		var rows []row
		err = db.NewRaw(fmt.Sprintf(`SELECT status, COUNT(*) AS n FROM %s.orders GROUP BY status ORDER BY 1`, s)).Scan(ctx, &rows)
		if err != nil {
			fmt.Printf("  ERR status dist: %v\n", err)
			continue
		}
		fmt.Printf("  status dist: %+v\n", rows)

		// order count + paid_amount backfill sanity
		var cnt int
		var paidAmt int64
		err = db.NewRaw(fmt.Sprintf(`SELECT COUNT(*) FROM %s.orders`, s)).Scan(ctx, &cnt)
		if err == nil {
			_ = db.NewRaw(fmt.Sprintf(`SELECT COALESCE(SUM(paid_amount),0) FROM %s.orders`, s)).Scan(ctx, &paidAmt)
			fmt.Printf("  orders: %d, sum paid_amount: %d\n", cnt, paidAmt)
		}
	}
}
