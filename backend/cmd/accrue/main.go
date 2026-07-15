// Command accrue runs the monthly leave accrual for every organization. Intended
// to be scheduled (e.g. a monthly cron). Idempotent per period.
//
// Usage:
//
//	go run ./cmd/accrue            # accrue the current month
//	go run ./cmd/accrue 2026-07    # accrue a specific month
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/audit"
	"github.com/gthimmes/we-people/backend/internal/config"
	"github.com/gthimmes/we-people/backend/internal/database"
	"github.com/gthimmes/we-people/backend/internal/timeoff"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "accrue error:", err)
		os.Exit(1)
	}
}

func run() error {
	period := time.Now().Format("2006-01")
	if len(os.Args) > 1 {
		period = os.Args[1]
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	svc := timeoff.NewService(timeoff.NewStore(conn.Pool), nil, audit.NewLogger(conn.Pool))

	// Accrue for every organization.
	rows, err := conn.Pool.Query(ctx, `SELECT id FROM organizations WHERE status = 'active'`)
	if err != nil {
		return err
	}
	var orgIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		orgIDs = append(orgIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, orgID := range orgIDs {
		res, err := svc.RunAccrual(ctx, orgID, period)
		if err != nil {
			return fmt.Errorf("org %s: %w", orgID, err)
		}
		fmt.Printf("org %s: credited %.2f hours to %d workers for %s\n", orgID, res.HoursCredited, res.WorkersCredited, res.Period)
	}
	return nil
}
