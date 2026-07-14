// Package orgstructure owns departments, locations, positions, worker
// assignments, and the reporting hierarchy (org chart).
package orgstructure

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

// Department is an organizational unit; departments form a tree via ParentID.
type Department struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	Name       string     `json:"name"`
	Code       string     `json:"code"`
	ParentID   *uuid.UUID `json:"parent_id,omitempty"`
	CostCenter string     `json:"cost_center"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Location is a physical/legal work site.
type Location struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	City      string    `json:"city"`
	Region    string    `json:"region"`
	Country   string    `json:"country"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
}

// Position is a specific seat that may be open, filled, or frozen.
type Position struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	Title         string     `json:"title"`
	DepartmentID  *uuid.UUID `json:"department_id,omitempty"`
	LocationID    *uuid.UUID `json:"location_id,omitempty"`
	JobProfileID  *uuid.UUID `json:"job_profile_id,omitempty"`
	LegalEntityID *uuid.UUID `json:"legal_entity_id,omitempty"`
	Status        string     `json:"status"`
	FTE           float64    `json:"fte"`
	CreatedAt     time.Time  `json:"created_at"`
}

// LegalEntity is a legal employer (subsidiary/entity) within an organization.
type LegalEntity struct {
	ID        uuid.UUID `json:"id"`
	OrgID     uuid.UUID `json:"org_id"`
	Name      string    `json:"name"`
	Country   string    `json:"country"`
	TaxID     string    `json:"tax_id"`
	CreatedAt time.Time `json:"created_at"`
}

// JobProfile is a reusable job definition (title, family, level, FLSA).
type JobProfile struct {
	ID         uuid.UUID `json:"id"`
	OrgID      uuid.UUID `json:"org_id"`
	Title      string    `json:"title"`
	JobFamily  string    `json:"job_family"`
	Level      string    `json:"level"`
	FLSAStatus string    `json:"flsa_status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Assignment links a worker to a position + manager, effective-dated.
type Assignment struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	WorkerID      uuid.UUID  `json:"worker_id"`
	PositionID    *uuid.UUID `json:"position_id,omitempty"`
	ManagerID     *uuid.UUID `json:"manager_id,omitempty"`
	EffectiveDate time.Time  `json:"effective_date"`
	EndDate       *time.Time `json:"end_date,omitempty"`
	IsPrimary     bool       `json:"is_primary"`
}

// Store provides org-scoped data access.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds an orgstructure Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the pool for transactional services.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// --- Departments ---

// CreateDepartment inserts a department.
func (s *Store) CreateDepartment(ctx context.Context, d Department) (Department, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO departments (org_id, name, code, parent_id, cost_center)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, org_id, name, code, parent_id, cost_center, created_at`,
		d.OrgID, d.Name, d.Code, d.ParentID, d.CostCenter).
		Scan(&d.ID, &d.OrgID, &d.Name, &d.Code, &d.ParentID, &d.CostCenter, &d.CreatedAt)
	return d, err
}

// ListDepartments returns all departments in an org.
func (s *Store) ListDepartments(ctx context.Context, orgID uuid.UUID) ([]Department, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, name, code, parent_id, cost_center, created_at
		FROM departments WHERE org_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Department
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.OrgID, &d.Name, &d.Code, &d.ParentID, &d.CostCenter, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// --- Locations ---

// CreateLocation inserts a location.
func (s *Store) CreateLocation(ctx context.Context, l Location) (Location, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO locations (org_id, name, address, city, region, country, timezone)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, org_id, name, address, city, region, country, timezone, created_at`,
		l.OrgID, l.Name, l.Address, l.City, l.Region, l.Country, l.Timezone).
		Scan(&l.ID, &l.OrgID, &l.Name, &l.Address, &l.City, &l.Region, &l.Country, &l.Timezone, &l.CreatedAt)
	return l, err
}

// ListLocations returns all locations in an org.
func (s *Store) ListLocations(ctx context.Context, orgID uuid.UUID) ([]Location, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, name, address, city, region, country, timezone, created_at
		FROM locations WHERE org_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.OrgID, &l.Name, &l.Address, &l.City, &l.Region, &l.Country, &l.Timezone, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// --- Positions ---

const positionCols = `id, org_id, title, department_id, location_id, job_profile_id,
	legal_entity_id, status, fte, created_at`

func scanPosition(row pgx.Row) (Position, error) {
	var p Position
	err := row.Scan(&p.ID, &p.OrgID, &p.Title, &p.DepartmentID, &p.LocationID,
		&p.JobProfileID, &p.LegalEntityID, &p.Status, &p.FTE, &p.CreatedAt)
	return p, err
}

// CreatePosition inserts a position.
func (s *Store) CreatePosition(ctx context.Context, p Position) (Position, error) {
	return scanPosition(s.pool.QueryRow(ctx, `
		INSERT INTO positions (org_id, title, department_id, location_id, job_profile_id, legal_entity_id, status, fte)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING `+positionCols,
		p.OrgID, p.Title, p.DepartmentID, p.LocationID, p.JobProfileID, p.LegalEntityID, p.Status, p.FTE))
}

// ListPositions returns all positions in an org.
func (s *Store) ListPositions(ctx context.Context, orgID uuid.UUID) ([]Position, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+positionCols+` FROM positions WHERE org_id = $1 ORDER BY title`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Position
	for rows.Next() {
		p, err := scanPosition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- Legal entities ---

// CreateLegalEntity inserts a legal entity.
func (s *Store) CreateLegalEntity(ctx context.Context, e LegalEntity) (LegalEntity, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO legal_entities (org_id, name, country, tax_id)
		VALUES ($1,$2,$3,$4)
		RETURNING id, org_id, name, country, tax_id, created_at`,
		e.OrgID, e.Name, e.Country, e.TaxID).
		Scan(&e.ID, &e.OrgID, &e.Name, &e.Country, &e.TaxID, &e.CreatedAt)
	return e, err
}

// ListLegalEntities returns all legal entities in an org.
func (s *Store) ListLegalEntities(ctx context.Context, orgID uuid.UUID) ([]LegalEntity, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, name, country, tax_id, created_at
		FROM legal_entities WHERE org_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LegalEntity
	for rows.Next() {
		var e LegalEntity
		if err := rows.Scan(&e.ID, &e.OrgID, &e.Name, &e.Country, &e.TaxID, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- Job profiles ---

// CreateJobProfile inserts a job profile.
func (s *Store) CreateJobProfile(ctx context.Context, j JobProfile) (JobProfile, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO job_profiles (org_id, title, job_family, level, flsa_status)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, org_id, title, job_family, level, flsa_status, created_at`,
		j.OrgID, j.Title, j.JobFamily, j.Level, j.FLSAStatus).
		Scan(&j.ID, &j.OrgID, &j.Title, &j.JobFamily, &j.Level, &j.FLSAStatus, &j.CreatedAt)
	return j, err
}

// ListJobProfiles returns all job profiles in an org.
func (s *Store) ListJobProfiles(ctx context.Context, orgID uuid.UUID) ([]JobProfile, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, title, job_family, level, flsa_status, created_at
		FROM job_profiles WHERE org_id = $1 ORDER BY title, level`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobProfile
	for rows.Next() {
		var j JobProfile
		if err := rows.Scan(&j.ID, &j.OrgID, &j.Title, &j.JobFamily, &j.Level, &j.FLSAStatus, &j.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// SetPositionStatusTx updates a position's status within a transaction.
func (s *Store) SetPositionStatusTx(ctx context.Context, tx pgx.Tx, orgID, id uuid.UUID, status string) error {
	_, err := tx.Exec(ctx, `UPDATE positions SET status=$3, updated_at=now() WHERE org_id=$1 AND id=$2`, orgID, id, status)
	return err
}

// --- Assignments ---

// CloseOpenPrimaryTx ends any open primary assignment for a worker as of the
// day before the new effective date. Used when transferring/reassigning.
func (s *Store) CloseOpenPrimaryTx(ctx context.Context, tx pgx.Tx, orgID, workerID uuid.UUID, endDate time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE worker_assignments SET end_date = $3
		WHERE org_id = $1 AND worker_id = $2 AND end_date IS NULL AND is_primary`,
		orgID, workerID, endDate)
	return err
}

// RecordLifecycleEventTx writes an employment event (e.g. transfer, promotion)
// into the shared lifecycle_events log within a transaction.
func (s *Store) RecordLifecycleEventTx(ctx context.Context, tx pgx.Tx, orgID, workerID uuid.UUID, eventType string, effectiveDate time.Time, reason string, createdBy *uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO lifecycle_events (org_id, worker_id, type, effective_date, reason, created_by)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		orgID, workerID, eventType, effectiveDate, reason, createdBy)
	return err
}

// CreateAssignmentTx inserts an assignment within a transaction.
func (s *Store) CreateAssignmentTx(ctx context.Context, tx pgx.Tx, a Assignment) (Assignment, error) {
	err := tx.QueryRow(ctx, `
		INSERT INTO worker_assignments (org_id, worker_id, position_id, manager_id, effective_date, end_date, is_primary)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, org_id, worker_id, position_id, manager_id, effective_date, end_date, is_primary`,
		a.OrgID, a.WorkerID, a.PositionID, a.ManagerID, a.EffectiveDate, a.EndDate, a.IsPrimary).
		Scan(&a.ID, &a.OrgID, &a.WorkerID, &a.PositionID, &a.ManagerID, &a.EffectiveDate, &a.EndDate, &a.IsPrimary)
	return a, err
}

const assignmentCols = `id, org_id, worker_id, position_id, manager_id, effective_date, end_date, is_primary`

func scanAssignment(row pgx.Row) (Assignment, error) {
	var a Assignment
	err := row.Scan(&a.ID, &a.OrgID, &a.WorkerID, &a.PositionID, &a.ManagerID,
		&a.EffectiveDate, &a.EndDate, &a.IsPrimary)
	return a, err
}

// ListAssignments returns a worker's full assignment history, newest first.
func (s *Store) ListAssignments(ctx context.Context, orgID, workerID uuid.UUID) ([]Assignment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+assignmentCols+` FROM worker_assignments
		WHERE org_id = $1 AND worker_id = $2
		ORDER BY effective_date DESC, created_at DESC`, orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Assignment
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AssignmentAsOf returns the primary assignment in effect on a given date —
// the effective-dated "what was true on date X" query.
func (s *Store) AssignmentAsOf(ctx context.Context, orgID, workerID uuid.UUID, asOf time.Time) (Assignment, error) {
	a, err := scanAssignment(s.pool.QueryRow(ctx, `
		SELECT `+assignmentCols+` FROM worker_assignments
		WHERE org_id = $1 AND worker_id = $2 AND is_primary
		  AND effective_date <= $3 AND (end_date IS NULL OR end_date > $3)
		ORDER BY effective_date DESC
		LIMIT 1`, orgID, workerID, asOf))
	if errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, ErrNotFound
	}
	return a, err
}

// OrgChartNode is a flattened row for building the reporting hierarchy: a
// worker plus their current manager (from their open primary assignment).
type OrgChartNode struct {
	WorkerID  uuid.UUID  `json:"worker_id"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	Title     *string    `json:"title,omitempty"`
	ManagerID *uuid.UUID `json:"manager_id,omitempty"`
}

// OrgChart returns one node per active worker with their current manager and
// position title, from which callers assemble the tree.
func (s *Store) OrgChart(ctx context.Context, orgID uuid.UUID) ([]OrgChartNode, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT w.id, w.first_name, w.last_name, p.title, a.manager_id
		FROM workers w
		LEFT JOIN worker_assignments a
			ON a.worker_id = w.id AND a.end_date IS NULL AND a.is_primary
		LEFT JOIN positions p ON p.id = a.position_id
		WHERE w.org_id = $1 AND w.status <> 'terminated'
		ORDER BY w.last_name, w.first_name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrgChartNode
	for rows.Next() {
		var n OrgChartNode
		if err := rows.Scan(&n.WorkerID, &n.FirstName, &n.LastName, &n.Title, &n.ManagerID); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
