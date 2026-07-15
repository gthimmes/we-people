// Package timeoff implements leave types, balances, and time-off requests. It is
// the first consumer of the generic approvals engine.
package timeoff

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when an entity does not exist in the org.
var ErrNotFound = errors.New("not found")

// LeaveType is a category of leave (Vacation, Sick, ...) with accrual config.
type LeaveType struct {
	ID                 uuid.UUID `json:"id"`
	OrgID              uuid.UUID `json:"org_id"`
	Name               string    `json:"name"`
	IsPaid             bool      `json:"is_paid"`
	AccrualEnabled     bool      `json:"accrual_enabled"`
	AccrualAnnualHours float64   `json:"accrual_annual_hours"`
	MaxBalanceHours    float64   `json:"max_balance_hours"`
	CarryoverMaxHours  *float64  `json:"carryover_max_hours,omitempty"` // nil = unlimited
}

// Balance is a worker's remaining hours for a leave type.
type Balance struct {
	LeaveTypeID   uuid.UUID `json:"leave_type_id"`
	LeaveTypeName string    `json:"leave_type_name"`
	BalanceHours  float64   `json:"balance_hours"`
}

// Request is a time-off request.
type Request struct {
	ID                uuid.UUID  `json:"id"`
	OrgID             uuid.UUID  `json:"org_id"`
	WorkerID          uuid.UUID  `json:"worker_id"`
	LeaveTypeID       uuid.UUID  `json:"leave_type_id"`
	LeaveTypeName     string     `json:"leave_type_name,omitempty"`
	StartDate         time.Time  `json:"start_date"`
	EndDate           time.Time  `json:"end_date"`
	Hours             float64    `json:"hours"`
	Reason            string     `json:"reason"`
	Status            string     `json:"status"`
	ApprovalRequestID *uuid.UUID `json:"approval_request_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

// Store provides org-scoped data access for time off.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds a timeoff Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the pool for transactional services.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// --- Leave types ---

const leaveTypeCols = `id, org_id, name, is_paid, accrual_enabled, accrual_annual_hours, max_balance_hours, carryover_max_hours`

func scanLeaveType(row pgx.Row) (LeaveType, error) {
	var lt LeaveType
	err := row.Scan(&lt.ID, &lt.OrgID, &lt.Name, &lt.IsPaid, &lt.AccrualEnabled,
		&lt.AccrualAnnualHours, &lt.MaxBalanceHours, &lt.CarryoverMaxHours)
	return lt, err
}

// CreateLeaveType inserts a leave type with its accrual config.
func (s *Store) CreateLeaveType(ctx context.Context, orgID uuid.UUID, lt LeaveType) (LeaveType, error) {
	return scanLeaveType(s.pool.QueryRow(ctx, `
		INSERT INTO leave_types (org_id, name, is_paid, accrual_enabled, accrual_annual_hours, max_balance_hours, carryover_max_hours)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING `+leaveTypeCols,
		orgID, lt.Name, lt.IsPaid, lt.AccrualEnabled, lt.AccrualAnnualHours, lt.MaxBalanceHours, lt.CarryoverMaxHours))
}

// ListLeaveTypes returns all leave types in an org.
func (s *Store) ListLeaveTypes(ctx context.Context, orgID uuid.UUID) ([]LeaveType, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+leaveTypeCols+` FROM leave_types WHERE org_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaveType
	for rows.Next() {
		lt, err := scanLeaveType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lt)
	}
	return out, rows.Err()
}

// UpdateLeaveType edits a leave type and its accrual config.
func (s *Store) UpdateLeaveType(ctx context.Context, orgID, id uuid.UUID, lt LeaveType) (LeaveType, error) {
	updated, err := scanLeaveType(s.pool.QueryRow(ctx, `
		UPDATE leave_types SET name=$3, is_paid=$4, accrual_enabled=$5,
			accrual_annual_hours=$6, max_balance_hours=$7, carryover_max_hours=$8
		WHERE org_id=$1 AND id=$2
		RETURNING `+leaveTypeCols,
		orgID, id, lt.Name, lt.IsPaid, lt.AccrualEnabled, lt.AccrualAnnualHours, lt.MaxBalanceHours, lt.CarryoverMaxHours))
	if errors.Is(err, pgx.ErrNoRows) {
		return LeaveType{}, ErrNotFound
	}
	return updated, err
}

// AccrualTypes returns leave types with accrual enabled and a positive rate.
func (s *Store) AccrualTypes(ctx context.Context, orgID uuid.UUID) ([]LeaveType, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+leaveTypeCols+` FROM leave_types WHERE org_id=$1 AND accrual_enabled AND accrual_annual_hours > 0`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaveType
	for rows.Next() {
		lt, err := scanLeaveType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lt)
	}
	return out, rows.Err()
}

// ActiveWorkerIDs returns the ids of active (non-terminated) workers.
func (s *Store) ActiveWorkerIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM workers WHERE org_id=$1 AND status <> 'terminated'`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// BalanceHours returns a worker's current balance for a leave type (0 if none).
func (s *Store) BalanceHours(ctx context.Context, tx pgx.Tx, orgID, workerID, leaveTypeID uuid.UUID) (float64, error) {
	var h float64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(balance_hours, 0) FROM leave_balances
		WHERE org_id=$1 AND worker_id=$2 AND leave_type_id=$3`, orgID, workerID, leaveTypeID).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return h, err
}

// RecordAccrualTx inserts a ledger entry, returning true only if it was new
// (the unique key makes re-runs of the same period a no-op).
func (s *Store) RecordAccrualTx(ctx context.Context, tx pgx.Tx, orgID, workerID, leaveTypeID uuid.UUID, period, kind string, hours float64) (bool, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO leave_accruals (org_id, worker_id, leave_type_id, period, kind, hours)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (org_id, worker_id, leave_type_id, period, kind) DO NOTHING
		RETURNING id`, orgID, workerID, leaveTypeID, period, kind, hours).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // already recorded this period
	}
	return err == nil, err
}

// DeleteLeaveType removes a leave type. Fails (FK) if requests reference it.
func (s *Store) DeleteLeaveType(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM leave_types WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetLeaveType returns a leave type by id in the org.
func (s *Store) GetLeaveType(ctx context.Context, orgID, id uuid.UUID) (LeaveType, error) {
	lt, err := scanLeaveType(s.pool.QueryRow(ctx,
		`SELECT `+leaveTypeCols+` FROM leave_types WHERE org_id=$1 AND id=$2`, orgID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return LeaveType{}, ErrNotFound
	}
	return lt, err
}

// --- Balances ---

// ListBalances returns a worker's leave balances joined with type names.
func (s *Store) ListBalances(ctx context.Context, orgID, workerID uuid.UUID) ([]Balance, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT lt.id, lt.name, COALESCE(b.balance_hours, 0)
		FROM leave_types lt
		LEFT JOIN leave_balances b
			ON b.leave_type_id = lt.id AND b.worker_id = $2 AND b.org_id = $1
		WHERE lt.org_id = $1
		ORDER BY lt.name`, orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Balance
	for rows.Next() {
		var b Balance
		if err := rows.Scan(&b.LeaveTypeID, &b.LeaveTypeName, &b.BalanceHours); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// AdjustBalanceTx changes a worker's balance by deltaHours (may be negative),
// creating the balance row if absent. Runs within a transaction.
func (s *Store) AdjustBalanceTx(ctx context.Context, tx pgx.Tx, orgID, workerID, leaveTypeID uuid.UUID, deltaHours float64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO leave_balances (org_id, worker_id, leave_type_id, balance_hours)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (org_id, worker_id, leave_type_id)
		DO UPDATE SET balance_hours = leave_balances.balance_hours + $4`,
		orgID, workerID, leaveTypeID, deltaHours)
	return err
}

// --- Requests ---

const reqCols = `id, org_id, worker_id, leave_type_id, start_date, end_date, hours, reason, status, approval_request_id, created_at`

func scanRequest(row pgx.Row) (Request, error) {
	var r Request
	err := row.Scan(&r.ID, &r.OrgID, &r.WorkerID, &r.LeaveTypeID, &r.StartDate, &r.EndDate,
		&r.Hours, &r.Reason, &r.Status, &r.ApprovalRequestID, &r.CreatedAt)
	return r, err
}

// CreateTx inserts a time-off request within a transaction.
func (s *Store) CreateTx(ctx context.Context, tx pgx.Tx, r Request) (Request, error) {
	return scanRequest(tx.QueryRow(ctx, `
		INSERT INTO time_off_requests (org_id, worker_id, leave_type_id, start_date, end_date, hours, reason, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'pending')
		RETURNING `+reqCols,
		r.OrgID, r.WorkerID, r.LeaveTypeID, r.StartDate, r.EndDate, r.Hours, r.Reason))
}

// SetApprovalRequestTx links an approval request to a time-off request.
func (s *Store) SetApprovalRequestTx(ctx context.Context, tx pgx.Tx, id, approvalID uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE time_off_requests SET approval_request_id=$2 WHERE id=$1`, id, approvalID)
	return err
}

// GetBySubjectTx returns a time-off request by id within a transaction.
func (s *Store) GetBySubjectTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Request, error) {
	r, err := scanRequest(tx.QueryRow(ctx, `SELECT `+reqCols+` FROM time_off_requests WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return r, err
}

// SetStatusTx sets a request's status within a transaction.
func (s *Store) SetStatusTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status string) error {
	_, err := tx.Exec(ctx, `UPDATE time_off_requests SET status=$2, decided_at=now() WHERE id=$1`, id, status)
	return err
}

// ListForWorker returns a worker's time-off requests, newest first.
func (s *Store) ListForWorker(ctx context.Context, orgID, workerID uuid.UUID) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixReqCols("t")+`, lt.name
		FROM time_off_requests t
		JOIN leave_types lt ON lt.id = t.leave_type_id
		WHERE t.org_id=$1 AND t.worker_id=$2
		ORDER BY t.created_at DESC`, orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Request
	for rows.Next() {
		var r Request
		if err := rows.Scan(&r.ID, &r.OrgID, &r.WorkerID, &r.LeaveTypeID, &r.StartDate, &r.EndDate,
			&r.Hours, &r.Reason, &r.Status, &r.ApprovalRequestID, &r.CreatedAt, &r.LeaveTypeName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// WorkerIDForUser returns the worker linked to a user, if any.
func (s *Store) WorkerIDForUser(ctx context.Context, orgID, userID uuid.UUID) (*uuid.UUID, error) {
	var wid *uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT worker_id FROM users WHERE org_id=$1 AND id=$2`, orgID, userID).Scan(&wid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return wid, err
}

// CurrentManager returns the manager on a worker's open primary assignment.
func (s *Store) CurrentManager(ctx context.Context, orgID, workerID uuid.UUID) (*uuid.UUID, error) {
	var mgr *uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT manager_id FROM worker_assignments
		WHERE org_id=$1 AND worker_id=$2 AND end_date IS NULL AND is_primary
		LIMIT 1`, orgID, workerID).Scan(&mgr)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return mgr, err
}

func prefixReqCols(alias string) string {
	cols := []string{"id", "org_id", "worker_id", "leave_type_id", "start_date", "end_date",
		"hours", "reason", "status", "approval_request_id", "created_at"}
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += alias + "." + c
	}
	return out
}
