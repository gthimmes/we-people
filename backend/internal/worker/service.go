package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/audit"
)

// Service implements worker business logic.
type Service struct {
	store *Store
	audit *audit.Logger
}

// NewService builds the worker service.
func NewService(store *Store, auditLog *audit.Logger) *Service {
	return &Service{store: store, audit: auditLog}
}

// CreateInput is the payload for hiring/creating a worker.
type CreateInput struct {
	EmployeeNumber string
	FirstName      string
	LastName       string
	PreferredName  string
	WorkEmail      string
	PersonalEmail  string
	Phone          string
	DateOfBirth    *time.Time
	HireDate       *time.Time
	Status         string
}

// Create inserts a new worker and records a hire lifecycle event, all in one
// transaction, then audit-logs the change.
func (s *Service) Create(ctx context.Context, orgID, actorUserID uuid.UUID, in CreateInput) (Worker, error) {
	status := in.Status
	if status == "" {
		status = "active"
	}
	wk := Worker{
		OrgID:          orgID,
		EmployeeNumber: in.EmployeeNumber,
		FirstName:      in.FirstName,
		LastName:       in.LastName,
		PreferredName:  in.PreferredName,
		WorkEmail:      in.WorkEmail,
		PersonalEmail:  in.PersonalEmail,
		Phone:          in.Phone,
		DateOfBirth:    in.DateOfBirth,
		HireDate:       in.HireDate,
		Status:         status,
	}

	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return Worker{}, err
	}
	defer tx.Rollback(ctx)

	created, err := s.store.Create(ctx, tx, wk)
	if err != nil {
		return Worker{}, fmt.Errorf("create worker: %w", err)
	}

	effective := time.Now()
	if in.HireDate != nil {
		effective = *in.HireDate
	}
	if err := s.store.LifecycleEvent(ctx, tx, orgID, created.ID, "hire", effective, "initial hire", &actorUserID); err != nil {
		return Worker{}, fmt.Errorf("record hire event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Worker{}, err
	}

	s.audit.Record(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: &actorUserID,
		Action: "worker.create", EntityType: "worker", EntityID: &created.ID,
		After: created,
	})
	return created, nil
}

// Get returns a worker by id within the org.
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (Worker, error) {
	return s.store.GetByID(ctx, orgID, id)
}

// List returns a page of workers plus the total count.
func (s *Service) List(ctx context.Context, p ListParams) ([]Worker, int, error) {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	workers, total, err := s.store.List(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	if workers == nil {
		workers = []Worker{}
	}
	return workers, total, nil
}

// UpdateInput carries editable worker fields.
type UpdateInput struct {
	FirstName     string
	LastName      string
	PreferredName string
	WorkEmail     string
	PersonalEmail string
	Phone         string
	DateOfBirth   *time.Time
	HireDate      *time.Time
	Status        string
}

// Update edits a worker and audit-logs the before/after.
func (s *Service) Update(ctx context.Context, orgID, actorUserID, id uuid.UUID, in UpdateInput) (Worker, error) {
	before, err := s.store.GetByID(ctx, orgID, id)
	if err != nil {
		return Worker{}, err
	}
	after := before
	after.FirstName = in.FirstName
	after.LastName = in.LastName
	after.PreferredName = in.PreferredName
	after.WorkEmail = in.WorkEmail
	after.PersonalEmail = in.PersonalEmail
	after.Phone = in.Phone
	after.DateOfBirth = in.DateOfBirth
	after.HireDate = in.HireDate
	if in.Status != "" {
		after.Status = in.Status
	}

	updated, err := s.store.Update(ctx, after)
	if err != nil {
		return Worker{}, err
	}
	s.audit.Record(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: &actorUserID,
		Action: "worker.update", EntityType: "worker", EntityID: &id,
		Before: before, After: updated,
	})
	return updated, nil
}
