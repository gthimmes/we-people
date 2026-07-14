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

// UpdateDepartment edits a department.
func (s *Service) UpdateDepartment(ctx context.Context, orgID, actor, id uuid.UUID, name, code string, parentID *uuid.UUID, costCenter string) (Department, error) {
	if parentID != nil && *parentID == id {
		return Department{}, ErrSelfParent
	}
	d, err := s.store.UpdateDepartment(ctx, Department{ID: id, OrgID: orgID, Name: name, Code: code, ParentID: parentID, CostCenter: costCenter})
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "department.update", EntityType: "department", EntityID: &id, After: d})
	}
	return d, err
}

// DeleteDepartment removes a department.
func (s *Service) DeleteDepartment(ctx context.Context, orgID, actor, id uuid.UUID) error {
	return s.deleteAndAudit(ctx, orgID, actor, id, "department", s.store.DeleteDepartment)
}

// UpdateLocation edits a location.
func (s *Service) UpdateLocation(ctx context.Context, orgID, actor uuid.UUID, l Location) (Location, error) {
	l.OrgID = orgID
	if l.Timezone == "" {
		l.Timezone = "UTC"
	}
	updated, err := s.store.UpdateLocation(ctx, l)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "location.update", EntityType: "location", EntityID: &l.ID, After: updated})
	}
	return updated, err
}

// DeleteLocation removes a location.
func (s *Service) DeleteLocation(ctx context.Context, orgID, actor, id uuid.UUID) error {
	return s.deleteAndAudit(ctx, orgID, actor, id, "location", s.store.DeleteLocation)
}

// UpdatePosition edits a position.
func (s *Service) UpdatePosition(ctx context.Context, orgID, actor uuid.UUID, p Position) (Position, error) {
	p.OrgID = orgID
	if p.Status == "" {
		p.Status = "open"
	}
	if p.FTE == 0 {
		p.FTE = 1.0
	}
	updated, err := s.store.UpdatePosition(ctx, p)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "position.update", EntityType: "position", EntityID: &p.ID, After: updated})
	}
	return updated, err
}

// DeletePosition removes a position.
func (s *Service) DeletePosition(ctx context.Context, orgID, actor, id uuid.UUID) error {
	return s.deleteAndAudit(ctx, orgID, actor, id, "position", s.store.DeletePosition)
}

// UpdateLegalEntity edits a legal entity.
func (s *Service) UpdateLegalEntity(ctx context.Context, orgID, actor uuid.UUID, e LegalEntity) (LegalEntity, error) {
	e.OrgID = orgID
	updated, err := s.store.UpdateLegalEntity(ctx, e)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "legal_entity.update", EntityType: "legal_entity", EntityID: &e.ID, After: updated})
	}
	return updated, err
}

// DeleteLegalEntity removes a legal entity.
func (s *Service) DeleteLegalEntity(ctx context.Context, orgID, actor, id uuid.UUID) error {
	return s.deleteAndAudit(ctx, orgID, actor, id, "legal_entity", s.store.DeleteLegalEntity)
}

// UpdateJobProfile edits a job profile.
func (s *Service) UpdateJobProfile(ctx context.Context, orgID, actor uuid.UUID, j JobProfile) (JobProfile, error) {
	j.OrgID = orgID
	if j.FLSAStatus == "" {
		j.FLSAStatus = "exempt"
	}
	updated, err := s.store.UpdateJobProfile(ctx, j)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "job_profile.update", EntityType: "job_profile", EntityID: &j.ID, After: updated})
	}
	return updated, err
}

// DeleteJobProfile removes a job profile.
func (s *Service) DeleteJobProfile(ctx context.Context, orgID, actor, id uuid.UUID) error {
	return s.deleteAndAudit(ctx, orgID, actor, id, "job_profile", s.store.DeleteJobProfile)
}

// deleteAndAudit runs a delete function and records an audit entry on success.
func (s *Service) deleteAndAudit(ctx context.Context, orgID, actor, id uuid.UUID, entity string, del func(context.Context, uuid.UUID, uuid.UUID) error) error {
	if err := del(ctx, orgID, id); err != nil {
		return err
	}
	s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: entity + ".delete", EntityType: entity, EntityID: &id})
	return nil
}

// CreateLegalEntity creates a legal entity.
func (s *Service) CreateLegalEntity(ctx context.Context, orgID, actor uuid.UUID, e LegalEntity) (LegalEntity, error) {
	e.OrgID = orgID
	created, err := s.store.CreateLegalEntity(ctx, e)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "legal_entity.create", EntityType: "legal_entity", EntityID: &created.ID, After: created})
	}
	return created, err
}

// ListLegalEntities returns legal entities (non-nil slice).
func (s *Service) ListLegalEntities(ctx context.Context, orgID uuid.UUID) ([]LegalEntity, error) {
	e, err := s.store.ListLegalEntities(ctx, orgID)
	if e == nil {
		e = []LegalEntity{}
	}
	return e, err
}

// CreateJobProfile creates a job profile.
func (s *Service) CreateJobProfile(ctx context.Context, orgID, actor uuid.UUID, j JobProfile) (JobProfile, error) {
	j.OrgID = orgID
	if j.FLSAStatus == "" {
		j.FLSAStatus = "exempt"
	}
	created, err := s.store.CreateJobProfile(ctx, j)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "job_profile.create", EntityType: "job_profile", EntityID: &created.ID, After: created})
	}
	return created, err
}

// ListJobProfiles returns job profiles (non-nil slice).
func (s *Service) ListJobProfiles(ctx context.Context, orgID uuid.UUID) ([]JobProfile, error) {
	j, err := s.store.ListJobProfiles(ctx, orgID)
	if j == nil {
		j = []JobProfile{}
	}
	return j, err
}

// ListAssignments returns a worker's assignment history (non-nil slice).
func (s *Service) ListAssignments(ctx context.Context, orgID, workerID uuid.UUID) ([]Assignment, error) {
	a, err := s.store.ListAssignments(ctx, orgID, workerID)
	if a == nil {
		a = []Assignment{}
	}
	return a, err
}

// AssignmentAsOf returns the assignment in effect on a given date.
func (s *Service) AssignmentAsOf(ctx context.Context, orgID, workerID uuid.UUID, asOf time.Time) (Assignment, error) {
	return s.store.AssignmentAsOf(ctx, orgID, workerID, asOf)
}

// AssignInput describes assigning a worker to a position and/or manager.
// EventType, when set (e.g. "transfer", "promotion"), records a matching
// lifecycle event so the change shows up on the worker's employment timeline.
type AssignInput struct {
	WorkerID      uuid.UUID
	PositionID    *uuid.UUID
	ManagerID     *uuid.UUID
	EffectiveDate time.Time
	EventType     string
	Reason        string
}

// Assign places a worker in a position under a manager. It closes any existing
// open primary assignment (preserving history), creates the new assignment,
// marks the target position filled, and optionally records a lifecycle event —
// all in one transaction.
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
	if in.EventType != "" {
		if err := s.store.RecordLifecycleEventTx(ctx, tx, orgID, in.WorkerID, in.EventType, in.EffectiveDate, in.Reason, &actor); err != nil {
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
