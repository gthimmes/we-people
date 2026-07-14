package approvals

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gthimmes/we-people/backend/internal/audit"
)

// Errors returned by the service.
var (
	ErrNotAuthorized = errors.New("not the assigned approver")
	ErrNotPending    = errors.New("request is not pending")
)

// Finalizer is implemented by a consuming module to apply the effect of a
// finalized (approved or rejected) request. Called inside the decision's
// transaction so the effect is atomic with the decision.
type Finalizer interface {
	OnApprovalFinalized(ctx context.Context, tx pgx.Tx, req Request) error
}

// Notifier delivers a notification to the user linked to a worker (best-effort).
type Notifier interface {
	NotifyWorker(ctx context.Context, orgID, workerID uuid.UUID, typ, title, body, link string)
}

// Service runs the approval engine.
type Service struct {
	store      *Store
	audit      *audit.Logger
	notifier   Notifier
	finalizers map[string]Finalizer
}

// NewService builds the approvals service.
func NewService(store *Store, auditLog *audit.Logger) *Service {
	return &Service{store: store, audit: auditLog, finalizers: map[string]Finalizer{}}
}

// SetNotifier wires the notification sink (optional; nil disables notifications).
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// RegisterFinalizer wires a consumer's effect handler for a request type. Called
// once at startup, after both services are constructed (breaks the cycle).
func (s *Service) RegisterFinalizer(requestType string, f Finalizer) {
	s.finalizers[requestType] = f
}

// CreateRequestTx creates an approval request with the given approver chain,
// within the caller's transaction. If there are no approvers the request is
// auto-approved and finalized immediately.
func (s *Service) CreateRequestTx(ctx context.Context, tx pgx.Tx, r Request, approvers []uuid.UUID) (Request, error) {
	r.Status = "pending"
	created, err := s.store.CreateRequestTx(ctx, tx, r, approvers)
	if err != nil {
		return Request{}, err
	}
	if len(approvers) == 0 {
		// Nothing to approve → auto-approve and run the effect now.
		if err := s.store.FinalizeRequestTx(ctx, tx, created.ID, "approved"); err != nil {
			return Request{}, err
		}
		created.Status = "approved"
		if err := s.finalize(ctx, tx, created); err != nil {
			return Request{}, err
		}
	}
	return created, nil
}

// Decide records an approver's approve/reject on the current step. adminOverride
// lets a privileged caller decide regardless of the assigned approver.
func (s *Service) Decide(ctx context.Context, orgID, actorUserID uuid.UUID, requestID uuid.UUID, approve bool, note string, adminOverride bool) (Request, error) {
	callerWorker, err := s.store.WorkerIDForUser(ctx, orgID, actorUserID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Request{}, err
	}

	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return Request{}, err
	}
	defer tx.Rollback(ctx)

	req, err := s.store.GetRequestForUpdateTx(ctx, tx, orgID, requestID)
	if err != nil {
		return Request{}, err
	}
	if req.Status != "pending" {
		return Request{}, ErrNotPending
	}
	step, err := s.store.CurrentStepTx(ctx, tx, req.ID, req.CurrentStep)
	if err != nil {
		return Request{}, err
	}

	// Authorization: assigned approver, or an admin override.
	if !adminOverride {
		if callerWorker == nil || step.ApproverWorkerID == nil || *callerWorker != *step.ApproverWorkerID {
			return Request{}, ErrNotAuthorized
		}
	}

	if approve {
		if err := s.store.DecideStepTx(ctx, tx, step.ID, "approved", note); err != nil {
			return Request{}, err
		}
		total, err := s.store.CountSteps(ctx, tx, req.ID)
		if err != nil {
			return Request{}, err
		}
		if req.CurrentStep >= total {
			if err := s.store.FinalizeRequestTx(ctx, tx, req.ID, "approved"); err != nil {
				return Request{}, err
			}
			req.Status = "approved"
			if err := s.finalize(ctx, tx, req); err != nil {
				return Request{}, err
			}
		} else {
			if err := s.store.AdvanceRequestTx(ctx, tx, req.ID, req.CurrentStep+1); err != nil {
				return Request{}, err
			}
			req.CurrentStep++
		}
	} else {
		if err := s.store.DecideStepTx(ctx, tx, step.ID, "rejected", note); err != nil {
			return Request{}, err
		}
		if err := s.store.FinalizeRequestTx(ctx, tx, req.ID, "rejected"); err != nil {
			return Request{}, err
		}
		req.Status = "rejected"
		if err := s.finalize(ctx, tx, req); err != nil {
			return Request{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Request{}, err
	}
	s.audit.Record(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: &actorUserID,
		Action: "approval.decide", EntityType: "approval_request", EntityID: &req.ID,
		After: map[string]any{"status": req.Status, "approve": approve},
	})
	// Notify the requester of the outcome once the decision is committed.
	if s.notifier != nil && req.RequesterWorkerID != nil && (req.Status == "approved" || req.Status == "rejected") {
		title := "Your request was " + req.Status
		s.notifier.NotifyWorker(ctx, orgID, *req.RequesterWorkerID, "approval.decided", title, note, "/time-off")
	}
	return req, nil
}

// CancelTx cancels a pending approval request within a caller's transaction.
// Used by a consumer when the underlying subject is withdrawn.
func (s *Service) CancelTx(ctx context.Context, tx pgx.Tx, orgID, requestID uuid.UUID) error {
	return s.store.CancelRequestTx(ctx, tx, orgID, requestID)
}

// finalize dispatches to the registered consumer effect for the request type.
func (s *Service) finalize(ctx context.Context, tx pgx.Tx, req Request) error {
	f, ok := s.finalizers[req.RequestType]
	if !ok {
		return fmt.Errorf("no finalizer registered for request type %q", req.RequestType)
	}
	return f.OnApprovalFinalized(ctx, tx, req)
}

// PendingForUser returns the approvals awaiting the caller — those assigned to
// their linked worker, plus (for admins) everything pending in the org.
func (s *Service) PendingForUser(ctx context.Context, orgID, userID uuid.UUID, adminOverride bool) ([]Request, error) {
	if adminOverride {
		reqs, err := s.store.PendingInOrg(ctx, orgID)
		return nonNil(reqs), err
	}
	worker, err := s.store.WorkerIDForUser(ctx, orgID, userID)
	if err != nil || worker == nil {
		return []Request{}, nil
	}
	reqs, err := s.store.PendingForApprover(ctx, orgID, *worker)
	return nonNil(reqs), err
}

func nonNil(r []Request) []Request {
	if r == nil {
		return []Request{}
	}
	return r
}
