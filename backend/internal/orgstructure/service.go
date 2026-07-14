package orgstructure

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/audit"
)

// Service implements orgstructure business logic.
type Service struct {
	store *Store
	audit *audit.Logger
}

// NewService builds the orgstructure service.
func NewService(store *Store, auditLog *audit.Logger) *Service {
	return &Service{store: store, audit: auditLog}
}

// CreateDepartment creates a department.
func (s *Service) CreateDepartment(ctx context.Context, orgID, actor uuid.UUID, name, code string, parentID *uuid.UUID, costCenter string) (Department, error) {
	d, err := s.store.CreateDepartment(ctx, Department{
		OrgID: orgID, Name: name, Code: code, ParentID: parentID, CostCenter: costCenter,
	})
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "department.create", EntityType: "department", EntityID: &d.ID, After: d})
	}
	return d, err
}

// ListDepartments returns departments (guaranteed non-nil slice).
func (s *Service) ListDepartments(ctx context.Context, orgID uuid.UUID) ([]Department, error) {
	d, err := s.store.ListDepartments(ctx, orgID)
	if d == nil {
		d = []Department{}
	}
	return d, err
}

// CreateLocation creates a location.
func (s *Service) CreateLocation(ctx context.Context, orgID, actor uuid.UUID, l Location) (Location, error) {
	l.OrgID = orgID
	if l.Timezone == "" {
		l.Timezone = "UTC"
	}
	created, err := s.store.CreateLocation(ctx, l)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "location.create", EntityType: "location", EntityID: &created.ID, After: created})
	}
	return created, err
}

// ListLocations returns locations (guaranteed non-nil slice).
func (s *Service) ListLocations(ctx context.Context, orgID uuid.UUID) ([]Location, error) {
	l, err := s.store.ListLocations(ctx, orgID)
	if l == nil {
		l = []Location{}
	}
	return l, err
}

// CreatePosition creates a position.
func (s *Service) CreatePosition(ctx context.Context, orgID, actor uuid.UUID, p Position) (Position, error) {
	p.OrgID = orgID
	if p.Status == "" {
		p.Status = "open"
	}
	if p.FTE == 0 {
		p.FTE = 1.0
	}
	created, err := s.store.CreatePosition(ctx, p)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "position.create", EntityType: "position", EntityID: &created.ID, After: created})
	}
	return created, err
}

// ListPositions returns positions (guaranteed non-nil slice).
func (s *Service) ListPositions(ctx context.Context, orgID uuid.UUID) ([]Position, error) {
	p, err := s.store.ListPositions(ctx, orgID)
	if p == nil {
		p = []Position{}
	}
	return p, err
}

// AssignInput describes assigning a worker to a position and/or manager.
type AssignInput struct {
	WorkerID      uuid.UUID
	PositionID    *uuid.UUID
	ManagerID     *uuid.UUID
	EffectiveDate time.Time
}

// Assign places a worker in a position under a manager. It closes any existing
// open primary assignment (preserving history), creates the new assignment, and
// marks the target position filled — all in one transaction.
func (s *Service) Assign(ctx context.Context, orgID, actor uuid.UUID, in AssignInput) (Assignment, error) {
	if in.EffectiveDate.IsZero() {
		in.EffectiveDate = time.Now()
	}
	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return Assignment{}, err
	}
	defer tx.Rollback(ctx)

	// End the prior open primary the day before the new one begins.
	if err := s.store.CloseOpenPrimaryTx(ctx, tx, orgID, in.WorkerID, in.EffectiveDate.AddDate(0, 0, -1)); err != nil {
		return Assignment{}, err
	}
	a, err := s.store.CreateAssignmentTx(ctx, tx, Assignment{
		OrgID:         orgID,
		WorkerID:      in.WorkerID,
		PositionID:    in.PositionID,
		ManagerID:     in.ManagerID,
		EffectiveDate: in.EffectiveDate,
		IsPrimary:     true,
	})
	if err != nil {
		return Assignment{}, err
	}
	if in.PositionID != nil {
		if err := s.store.SetPositionStatusTx(ctx, tx, orgID, *in.PositionID, "filled"); err != nil {
			return Assignment{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, err
	}
	s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "assignment.create", EntityType: "assignment", EntityID: &a.ID, After: a})
	return a, nil
}

// OrgChartTreeNode is a node in the assembled reporting hierarchy.
type OrgChartTreeNode struct {
	OrgChartNode
	Reports []*OrgChartTreeNode `json:"reports"`
}

// OrgChart returns the reporting hierarchy as a forest of roots (workers with
// no manager, or whose manager is outside the active set).
func (s *Service) OrgChart(ctx context.Context, orgID uuid.UUID) ([]*OrgChartTreeNode, error) {
	nodes, err := s.store.OrgChart(ctx, orgID)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*OrgChartTreeNode, len(nodes))
	for _, n := range nodes {
		byID[n.WorkerID] = &OrgChartTreeNode{OrgChartNode: n, Reports: []*OrgChartTreeNode{}}
	}
	roots := []*OrgChartTreeNode{}
	for _, node := range byID {
		if node.ManagerID != nil {
			if mgr, ok := byID[*node.ManagerID]; ok {
				mgr.Reports = append(mgr.Reports, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots, nil
}
