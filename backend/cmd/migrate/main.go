// Command migrate applies or rolls back database migrations.
//
// Usage:
//
//	go run ./cmd/migrate up      # apply all pending migrations
//	go run ./cmd/migrate down    # roll back the most recent migration
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gthimmes/we-people/backend/db"
	"github.com/gthimmes/we-people/backend/internal/config"
	"github.com/gthimmes/we-people/backend/internal/database"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down>")
		os.Exit(2)
	}
	cmd := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		fatal("load config", err)
	}
	ctx := context.Background()
	conn, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("connect database", err)
	}
	defer conn.Close()

	migrations, err := database.LoadMigrations(db.MigrationsFS())
	if err != nil {
		fatal("load migrations", err)
	}

	switch cmd {
	case "up":
		if err := database.MigrateUp(ctx, conn.Pool, migrations); err != nil {
			fatal("migrate up", err)
		}
		fmt.Printf("applied migrations (%d defined)\n", len(migrations))
	case "down":
		if err := database.MigrateDown(ctx, conn.Pool, migrations); err != nil {
			if errors.Is(err, database.ErrNoMigrations) {
				fmt.Println("nothing to roll back")
				return
			}
			fatal("migrate down", err)
		}
		fmt.Println("rolled back one migration")
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (want up|down)\n", cmd)
		os.Exit(2)
	}
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}
