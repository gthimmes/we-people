package timeoff

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gthimmes/we-people/backend/internal/approvals"
	"github.com/gthimmes/we-people/backend/internal/audit"
)

// Service errors.
var (
	ErrInvalidLeaveType = errors.New("invalid leave type")
	ErrNoWorker         = errors.New("no worker linked to this user; specify worker_id")
)

// Service implements time-off business logic and acts as the approvals
// Finalizer for the "time_off" request type.
type Service struct {
	store     *Store
	approvals *approvals.Service
	audit     *audit.Logger
}

// NewService builds the time-off service.
func NewService(store *Store, approvalsSvc *approvals.Service, auditLog *audit.Logger) *Service {
	return &Service{store: store, approvals: approvalsSvc, audit: auditLog}
}

// RequestType is the approvals request type this module owns.
const RequestType = "time_off"

// CreateInput is the payload for requesting time off.
type CreateInput struct {
	WorkerID    uuid.UUID
	LeaveTypeID uuid.UUID
	StartDate   time.Time
	EndDate     time.Time
	Hours       float64
	Reason      string
}

// Create files a time-off request and opens an approval routed to the worker's
// current manager. With no manager the request is auto-approved.
func (s *Service) Create(ctx context.Context, orgID, actorUserID uuid.UUID, in CreateInput) (Request, error) {
	if _, err := s.store.GetLeaveType(ctx, orgID, in.LeaveTypeID); err != nil {
		return Request{}, ErrInvalidLeaveType
	}

	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return Request{}, err
	}
	defer tx.Rollback(ctx)

	created, err := s.store.CreateTx(ctx, tx, Request{
		OrgID: orgID, WorkerID: in.WorkerID, LeaveTypeID: in.LeaveTypeID,
		StartDate: in.StartDate, EndDate: in.EndDate, Hours: in.Hours, Reason: in.Reason,
	})
	if err != nil {
		return Request{}, fmt.Errorf("create time-off: %w", err)
	}

	// Route to the worker's current manager, if any.
	manager, err := s.store.CurrentManager(ctx, orgID, in.WorkerID)
	if err != nil {
		return Request{}, err
	}
	var approvers []uuid.UUID
	if manager != nil {
		approvers = []uuid.UUID{*manager}
	}

	workerID := in.WorkerID
	appReq, err := s.approvals.CreateRequestTx(ctx, tx, approvals.Request{
		OrgID:             orgID,
		RequestType:       RequestType,
		SubjectType:       "time_off_request",
		SubjectID:         created.ID,
		RequesterWorkerID: &workerID,
	}, approvers)
	if err != nil {
		return Request{}, fmt.Errorf("open approval: %w", err)
	}
	if err := s.store.SetApprovalRequestTx(ctx, tx, created.ID, appReq.ID); err != nil {
		return Request{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Request{}, err
	}

	s.audit.Record(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: &actorUserID,
		Action: "time_off.request", EntityType: "time_off_request", EntityID: &created.ID,
		After: created,
	})
	// Reflect any auto-approval that happened during creation.
	if appReq.Status == "approved" {
		created.Status = "approved"
	}
	return created, nil
}

// OnApprovalFinalized applies a decision to the underlying time-off request.
// Implements approvals.Finalizer; runs inside the decision's transaction.
func (s *Service) OnApprovalFinalized(ctx context.Context, tx pgx.Tx, req approvals.Request) error {
	to, err := s.store.GetBySubjectTx(ctx, tx, req.SubjectID)
	if err != nil {
		return err
	}
	switch req.Status {
	case "approved":
		if err := s.store.SetStatusTx(ctx, tx, to.ID, "approved"); err != nil {
			return err
		}
		// Deduct the hours from the worker's balance.
		return s.store.AdjustBalanceTx(ctx, tx, to.OrgID, to.WorkerID, to.LeaveTypeID, -to.Hours)
	case "rejected":
		return s.store.SetStatusTx(ctx, tx, to.ID, "rejected")
	default:
		return nil
	}
}

// ListLeaveTypes returns the org's leave types (non-nil slice).
func (s *Service) ListLeaveTypes(ctx context.Context, orgID uuid.UUID) ([]LeaveType, error) {
	lt, err := s.store.ListLeaveTypes(ctx, orgID)
	if lt == nil {
		lt = []LeaveType{}
	}
	return lt, err
}

// Balances returns a worker's leave balances (non-nil slice).
func (s *Service) Balances(ctx context.Context, orgID, workerID uuid.UUID) ([]Balance, error) {
	b, err := s.store.ListBalances(ctx, orgID, workerID)
	if b == nil {
		b = []Balance{}
	}
	return b, err
}

// ListForWorker returns a worker's time-off requests (non-nil slice).
func (s *Service) ListForWorker(ctx context.Context, orgID, workerID uuid.UUID) ([]Request, error) {
	r, err := s.store.ListForWorker(ctx, orgID, workerID)
	if r == nil {
		r = []Request{}
	}
	return r, err
}

// EnsureLeaveType creates a leave type (used by seeding/admin).
func (s *Service) EnsureLeaveType(ctx context.Context, orgID uuid.UUID, name string, isPaid bool) (LeaveType, error) {
	return s.store.CreateLeaveType(ctx, orgID, name, isPaid)
}

// GrantBalance adds hours to a worker's balance for a leave type (seeding/admin).
func (s *Service) GrantBalance(ctx context.Context, orgID, workerID, leaveTypeID uuid.UUID, hours float64) error {
	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.store.AdjustBalanceTx(ctx, tx, orgID, workerID, leaveTypeID, hours); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ResolveWorker returns explicit if provided, else the caller's linked worker.
func (s *Service) ResolveWorker(ctx context.Context, orgID, userID uuid.UUID, explicit *uuid.UUID) (uuid.UUID, error) {
	if explicit != nil {
		return *explicit, nil
	}
	w, err := s.store.WorkerIDForUser(ctx, orgID, userID)
	if err != nil {
		return uuid.Nil, err
	}
	if w == nil {
		return uuid.Nil, ErrNoWorker
	}
	return *w, nil
}
