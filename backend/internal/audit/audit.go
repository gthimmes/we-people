// Package audit records immutable change events for compliance and debugging.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Logger writes audit events.
type Logger struct {
	pool *pgxpool.Pool
}

// NewLogger builds an audit Logger.
func NewLogger(pool *pgxpool.Pool) *Logger { return &Logger{pool: pool} }

// Entry describes a single auditable change.
type Entry struct {
	OrgID       uuid.UUID
	ActorUserID *uuid.UUID
	Action      string // e.g. "worker.create"
	EntityType  string // e.g. "worker"
	EntityID    *uuid.UUID
	Before      any
	After       any
}

// Record persists an audit entry. Failures are logged but never block the
// caller's operation — auditing must not take down a write path.
func (l *Logger) Record(ctx context.Context, e Entry) {
	before, err := marshal(e.Before)
	if err != nil {
		slog.Error("audit marshal before", "error", err)
	}
	after, err := marshal(e.After)
	if err != nil {
		slog.Error("audit marshal after", "error", err)
	}
	_, err = l.pool.Exec(ctx, `
		INSERT INTO audit_events (org_id, actor_user_id, action, entity_type, entity_id, before, after)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.OrgID, e.ActorUserID, e.Action, e.EntityType, e.EntityID, before, after)
	if err != nil {
		slog.Error("audit record", "error", err, "action", e.Action)
	}
}

func marshal(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
