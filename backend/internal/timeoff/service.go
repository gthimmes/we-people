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
	ErrNotCancellable   = errors.New("only pending requests can be cancelled")
	ErrNotOwner         = errors.New("you can only cancel your own requests")
)

// Notifier delivers a notification to the user linked to a worker (best-effort).
type Notifier interface {
	NotifyWorker(ctx context.Context, orgID, workerID uuid.UUID, typ, title, body, link string)
}

// Service implements time-off business logic and acts as the approvals
// Finalizer for the "time_off" request type.
type Service struct {
	store     *Store
	approvals *approvals.Service
	audit     *audit.Logger
	notifier  Notifier
}

// NewService builds the time-off service.
func NewService(store *Store, approvalsSvc *approvals.Service, auditLog *audit.Logger) *Service {
	return &Service{store: store, approvals: approvalsSvc, audit: auditLog}
}

// SetNotifier wires the notification sink (optional).
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

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
	// Notify the approving manager that a request awaits them.
	if s.notifier != nil && manager != nil && appReq.Status == "pending" {
		s.notifier.NotifyWorker(ctx, orgID, *manager, "approval.assigned",
			"New time-off request to review", in.Reason, "/time-off")
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

// EnsureLeaveType creates a basic leave type (used by seeding).
func (s *Service) EnsureLeaveType(ctx context.Context, orgID uuid.UUID, name string, isPaid bool) (LeaveType, error) {
	return s.store.CreateLeaveType(ctx, orgID, LeaveType{Name: name, IsPaid: isPaid})
}

// CreateLeaveType creates a leave type with accrual config.
func (s *Service) CreateLeaveType(ctx context.Context, orgID uuid.UUID, lt LeaveType) (LeaveType, error) {
	return s.store.CreateLeaveType(ctx, orgID, lt)
}

// UpdateLeaveType edits a leave type and its accrual config.
func (s *Service) UpdateLeaveType(ctx context.Context, orgID, id uuid.UUID, lt LeaveType) (LeaveType, error) {
	return s.store.UpdateLeaveType(ctx, orgID, id, lt)
}

// AccrualResult summarizes an accrual run.
type AccrualResult struct {
	Period          string  `json:"period"`
	WorkersCredited int     `json:"workers_credited"`
	HoursCredited   float64 `json:"hours_credited"`
}

// RunAccrual credits one period's accrual to every active worker for each
// accrual-enabled leave type, respecting balance caps. Idempotent per period.
func (s *Service) RunAccrual(ctx context.Context, orgID uuid.UUID, period string) (AccrualResult, error) {
	types, err := s.store.AccrualTypes(ctx, orgID)
	if err != nil {
		return AccrualResult{}, err
	}
	workers, err := s.store.ActiveWorkerIDs(ctx, orgID)
	if err != nil {
		return AccrualResult{}, err
	}

	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return AccrualResult{}, err
	}
	defer tx.Rollback(ctx)

	res := AccrualResult{Period: period}
	credited := map[uuid.UUID]bool{}
	for _, lt := range types {
		monthly := lt.AccrualAnnualHours / 12
		for _, wid := range workers {
			current, err := s.store.BalanceHours(ctx, tx, orgID, wid, lt.ID)
			if err != nil {
				return AccrualResult{}, err
			}
			delta := monthly
			if lt.MaxBalanceHours > 0 {
				room := lt.MaxBalanceHours - current
				if room <= 0 {
					continue // at or above cap; accrual paused
				}
				if delta > room {
					delta = room
				}
			}
			if delta <= 0 {
				continue
			}
			inserted, err := s.store.RecordAccrualTx(ctx, tx, orgID, wid, lt.ID, period, "accrual", delta)
			if err != nil {
				return AccrualResult{}, err
			}
			if inserted {
				if err := s.store.AdjustBalanceTx(ctx, tx, orgID, wid, lt.ID, delta); err != nil {
					return AccrualResult{}, err
				}
				res.HoursCredited += delta
				credited[wid] = true
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return AccrualResult{}, err
	}
	res.WorkersCredited = len(credited)
	return res, nil
}

// RunCarryover forfeits balance above each leave type's carryover cap at a year
// boundary. Leave types with an unlimited (null) cap are untouched. Idempotent.
func (s *Service) RunCarryover(ctx context.Context, orgID uuid.UUID, year int) (AccrualResult, error) {
	types, err := s.store.ListLeaveTypes(ctx, orgID)
	if err != nil {
		return AccrualResult{}, err
	}
	workers, err := s.store.ActiveWorkerIDs(ctx, orgID)
	if err != nil {
		return AccrualResult{}, err
	}
	period := fmt.Sprintf("%d-CO", year)

	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return AccrualResult{}, err
	}
	defer tx.Rollback(ctx)

	res := AccrualResult{Period: period}
	affected := map[uuid.UUID]bool{}
	for _, lt := range types {
		if lt.CarryoverMaxHours == nil {
			continue // unlimited carryover
		}
		cap := *lt.CarryoverMaxHours
		for _, wid := range workers {
			current, err := s.store.BalanceHours(ctx, tx, orgID, wid, lt.ID)
			if err != nil {
				return AccrualResult{}, err
			}
			if current <= cap {
				continue
			}
			delta := cap - current // negative: forfeit the excess
			inserted, err := s.store.RecordAccrualTx(ctx, tx, orgID, wid, lt.ID, period, "carryover", delta)
			if err != nil {
				return AccrualResult{}, err
			}
			if inserted {
				if err := s.store.AdjustBalanceTx(ctx, tx, orgID, wid, lt.ID, delta); err != nil {
					return AccrualResult{}, err
				}
				res.HoursCredited += delta
				affected[wid] = true
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return AccrualResult{}, err
	}
	res.WorkersCredited = len(affected)
	return res, nil
}

// DeleteLeaveType removes a leave type.
func (s *Service) DeleteLeaveType(ctx context.Context, orgID, id uuid.UUID) error {
	return s.store.DeleteLeaveType(ctx, orgID, id)
}

// Cancel withdraws a pending time-off request and cancels its approval so it
// leaves the approver's inbox. The caller must own the request (or be an admin).
func (s *Service) Cancel(ctx context.Context, orgID, actorUserID, requestID uuid.UUID, adminOverride bool) error {
	callerWorker, err := s.store.WorkerIDForUser(ctx, orgID, actorUserID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	req, err := s.store.GetBySubjectTx(ctx, tx, requestID)
	if err != nil {
		return err
	}
	if req.OrgID != orgID {
		return ErrNotFound
	}
	if !adminOverride && (callerWorker == nil || *callerWorker != req.WorkerID) {
		return ErrNotOwner
	}
	if req.Status != "pending" {
		return ErrNotCancellable
	}
	if err := s.store.SetStatusTx(ctx, tx, req.ID, "cancelled"); err != nil {
		return err
	}
	if req.ApprovalRequestID != nil {
		if err := s.approvals.CancelTx(ctx, tx, orgID, *req.ApprovalRequestID); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.audit.Record(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: &actorUserID,
		Action: "time_off.cancel", EntityType: "time_off_request", EntityID: &req.ID,
	})
	return nil
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
