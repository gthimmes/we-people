// Package approvals is a generic, reusable approval engine: a request routed
// through one or more approver steps. Consumers (time off, offers, comp changes)
// create requests and register a Finalizer to apply the decision's effect.
package approvals

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a request or step does not exist.
var ErrNotFound = errors.New("approval not found")

// Request is an approval request routed through Steps.
type Request struct {
	ID                uuid.UUID  `json:"id"`
	OrgID             uuid.UUID  `json:"org_id"`
	RequestType       string     `json:"request_type"`
	SubjectType       string     `json:"subject_type"`
	SubjectID         uuid.UUID  `json:"subject_id"`
	RequesterWorkerID *uuid.UUID `json:"requester_worker_id,omitempty"`
	Status            string     `json:"status"`
	CurrentStep       int        `json:"current_step"`
	CreatedAt         time.Time  `json:"created_at"`
	DecidedAt         *time.Time `json:"decided_at,omitempty"`
}

// Step is one approver's decision point in a Request.
type Step struct {
	ID               uuid.UUID  `json:"id"`
	RequestID        uuid.UUID  `json:"request_id"`
	Sequence         int        `json:"sequence"`
	ApproverWorkerID *uuid.UUID `json:"approver_worker_id,omitempty"`
	Status           string     `json:"status"`
	Note             string     `json:"note"`
	DecidedAt        *time.Time `json:"decided_at,omitempty"`
}

// Store provides org-scoped data access for approvals.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds an approvals Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the pool for transactional callers.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

const requestCols = `id, org_id, request_type, subject_type, subject_id,
	requester_worker_id, status, current_step, created_at, decided_at`

func scanRequest(row pgx.Row) (Request, error) {
	var r Request
	err := row.Scan(&r.ID, &r.OrgID, &r.RequestType, &r.SubjectType, &r.SubjectID,
		&r.RequesterWorkerID, &r.Status, &r.CurrentStep, &r.CreatedAt, &r.DecidedAt)
	return r, err
}

// CreateRequestTx inserts a request and its ordered steps within a transaction.
func (s *Store) CreateRequestTx(ctx context.Context, tx pgx.Tx, r Request, approverWorkerIDs []uuid.UUID) (Request, error) {
	created, err := scanRequest(tx.QueryRow(ctx, `
		INSERT INTO approval_requests (org_id, request_type, subject_type, subject_id, requester_worker_id, status, current_step)
		VALUES ($1,$2,$3,$4,$5,$6,1)
		RETURNING `+requestCols,
		r.OrgID, r.RequestType, r.SubjectType, r.SubjectID, r.RequesterWorkerID, r.Status))
	if err != nil {
		return Request{}, err
	}
	for i, approver := range approverWorkerIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_steps (request_id, sequence, approver_worker_id)
			VALUES ($1,$2,$3)`, created.ID, i+1, approver); err != nil {
			return Request{}, err
		}
	}
	return created, nil
}

// GetRequest returns a request by id, scoped to the org.
func (s *Store) GetRequest(ctx context.Context, orgID, id uuid.UUID) (Request, error) {
	r, err := scanRequest(s.pool.QueryRow(ctx,
		`SELECT `+requestCols+` FROM approval_requests WHERE org_id=$1 AND id=$2`, orgID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return r, err
}

// GetRequestForUpdateTx locks and returns a request within a transaction.
func (s *Store) GetRequestForUpdateTx(ctx context.Context, tx pgx.Tx, orgID, id uuid.UUID) (Request, error) {
	r, err := scanRequest(tx.QueryRow(ctx,
		`SELECT `+requestCols+` FROM approval_requests WHERE org_id=$1 AND id=$2 FOR UPDATE`, orgID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return r, err
}

// CurrentStepTx returns the pending step at the request's current sequence.
func (s *Store) CurrentStepTx(ctx context.Context, tx pgx.Tx, requestID uuid.UUID, sequence int) (Step, error) {
	var st Step
	err := tx.QueryRow(ctx, `
		SELECT id, request_id, sequence, approver_worker_id, status, note, decided_at
		FROM approval_steps WHERE request_id=$1 AND sequence=$2`, requestID, sequence).
		Scan(&st.ID, &st.RequestID, &st.Sequence, &st.ApproverWorkerID, &st.Status, &st.Note, &st.DecidedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Step{}, ErrNotFound
	}
	return st, err
}

// DecideStepTx records an approver's decision on a step.
func (s *Store) DecideStepTx(ctx context.Context, tx pgx.Tx, stepID uuid.UUID, status, note string) error {
	_, err := tx.Exec(ctx, `
		UPDATE approval_steps SET status=$2, note=$3, decided_at=now() WHERE id=$1`,
		stepID, status, note)
	return err
}

// AdvanceRequestTx sets the request's current step.
func (s *Store) AdvanceRequestTx(ctx context.Context, tx pgx.Tx, requestID uuid.UUID, currentStep int) error {
	_, err := tx.Exec(ctx, `UPDATE approval_requests SET current_step=$2 WHERE id=$1`, requestID, currentStep)
	return err
}

// FinalizeRequestTx sets a terminal status and decided_at.
func (s *Store) FinalizeRequestTx(ctx context.Context, tx pgx.Tx, requestID uuid.UUID, status string) error {
	_, err := tx.Exec(ctx, `UPDATE approval_requests SET status=$2, decided_at=now() WHERE id=$1`, requestID, status)
	return err
}

// CountSteps returns the number of steps on a request.
func (s *Store) CountSteps(ctx context.Context, tx pgx.Tx, requestID uuid.UUID) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM approval_steps WHERE request_id=$1`, requestID).Scan(&n)
	return n, err
}

// PendingForApprover returns requests currently awaiting a given approver.
func (s *Store) PendingForApprover(ctx context.Context, orgID, approverWorkerID uuid.UUID) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+prefixedReq()+`
		FROM approval_requests r
		JOIN approval_steps st
			ON st.request_id = r.id AND st.sequence = r.current_step
		WHERE r.org_id=$1 AND r.status='pending' AND st.status='pending'
		  AND st.approver_worker_id=$2
		ORDER BY r.created_at`, orgID, approverWorkerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRequests(rows)
}

// PendingInOrg returns all pending requests in the org (admin override view).
func (s *Store) PendingInOrg(ctx context.Context, orgID uuid.UUID) ([]Request, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+requestCols+` FROM approval_requests WHERE org_id=$1 AND status='pending' ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRequests(rows)
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

func prefixedReq() string {
	// requestCols with an "r." alias for the join query.
	return `r.id, r.org_id, r.request_type, r.subject_type, r.subject_id,
		r.requester_worker_id, r.status, r.current_step, r.created_at, r.decided_at`
}

func collectRequests(rows pgx.Rows) ([]Request, error) {
	var out []Request
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
