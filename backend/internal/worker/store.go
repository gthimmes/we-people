// Package worker owns worker (employee) records and employment lifecycle events.
package worker

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a worker does not exist in the org.
var ErrNotFound = errors.New("worker not found")

// Worker is a person the organization employs (or did / will).
type Worker struct {
	ID             uuid.UUID  `json:"id"`
	OrgID          uuid.UUID  `json:"org_id"`
	EmployeeNumber string     `json:"employee_number"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	PreferredName  string     `json:"preferred_name"`
	WorkEmail      string     `json:"work_email"`
	PersonalEmail  string     `json:"personal_email"`
	Phone          string     `json:"phone"`
	DateOfBirth    *time.Time `json:"date_of_birth,omitempty"`
	HireDate       *time.Time `json:"hire_date,omitempty"`
	Status         string     `json:"status"`
	// Home address
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	City         string `json:"city"`
	Region       string `json:"region"`
	PostalCode   string `json:"postal_code"`
	Country      string `json:"country"`
	// Demographics (EEO)
	Gender        string `json:"gender"`
	Ethnicity     string `json:"ethnicity"`
	MaritalStatus string `json:"marital_status"`
	// Work eligibility / I-9
	WorkAuthType   string     `json:"work_auth_type"`
	WorkAuthExpiry *time.Time `json:"work_auth_expiry,omitempty"`
	I9Verified     bool       `json:"i9_verified"`
	I9VerifiedOn   *time.Time `json:"i9_verified_on,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Store provides data access for workers, scoped by org.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds a worker Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the pool for transactional services.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

const workerCols = `id, org_id, employee_number, first_name, last_name, preferred_name,
	work_email, personal_email, phone, date_of_birth, hire_date, status,
	address_line1, address_line2, city, region, postal_code, country,
	gender, ethnicity, marital_status,
	work_auth_type, work_auth_expiry, i9_verified, i9_verified_on,
	created_at, updated_at`

// prefixed rewrites a comma-separated column list so each column carries the
// given table alias, e.g. prefixed("w", "id, name") -> "w.id, w.name".
func prefixed(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}

func scanWorker(row pgx.Row) (Worker, error) {
	var wk Worker
	err := row.Scan(&wk.ID, &wk.OrgID, &wk.EmployeeNumber, &wk.FirstName, &wk.LastName,
		&wk.PreferredName, &wk.WorkEmail, &wk.PersonalEmail, &wk.Phone,
		&wk.DateOfBirth, &wk.HireDate, &wk.Status,
		&wk.AddressLine1, &wk.AddressLine2, &wk.City, &wk.Region, &wk.PostalCode, &wk.Country,
		&wk.Gender, &wk.Ethnicity, &wk.MaritalStatus,
		&wk.WorkAuthType, &wk.WorkAuthExpiry, &wk.I9Verified, &wk.I9VerifiedOn,
		&wk.CreatedAt, &wk.UpdatedAt)
	return wk, err
}

// Create inserts a worker within a transaction.
func (s *Store) Create(ctx context.Context, tx pgx.Tx, wk Worker) (Worker, error) {
	return scanWorker(tx.QueryRow(ctx, `
		INSERT INTO workers (org_id, employee_number, first_name, last_name, preferred_name,
			work_email, personal_email, phone, date_of_birth, hire_date, status,
			address_line1, address_line2, city, region, postal_code, country,
			gender, ethnicity, marital_status,
			work_auth_type, work_auth_expiry, i9_verified, i9_verified_on)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)
		RETURNING `+workerCols,
		wk.OrgID, wk.EmployeeNumber, wk.FirstName, wk.LastName, wk.PreferredName,
		wk.WorkEmail, wk.PersonalEmail, wk.Phone, wk.DateOfBirth, wk.HireDate, wk.Status,
		wk.AddressLine1, wk.AddressLine2, wk.City, wk.Region, wk.PostalCode, wk.Country,
		wk.Gender, wk.Ethnicity, wk.MaritalStatus,
		wk.WorkAuthType, wk.WorkAuthExpiry, wk.I9Verified, wk.I9VerifiedOn))
}

// GetByID returns a worker by id, scoped to the org.
func (s *Store) GetByID(ctx context.Context, orgID, id uuid.UUID) (Worker, error) {
	wk, err := scanWorker(s.pool.QueryRow(ctx,
		`SELECT `+workerCols+` FROM workers WHERE org_id = $1 AND id = $2`, orgID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Worker{}, ErrNotFound
	}
	return wk, err
}

// ListParams controls a worker listing.
type ListParams struct {
	OrgID  uuid.UUID
	Search string
	Status string
	Limit  int
	Offset int
}

// List returns workers in an org plus the total count matching the filter.
func (s *Store) List(ctx context.Context, p ListParams) ([]Worker, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+workerCols+` FROM workers
		WHERE org_id = $1
		  AND ($2 = '' OR (first_name ILIKE '%'||$2||'%' OR last_name ILIKE '%'||$2||'%'
		                   OR work_email ILIKE '%'||$2||'%' OR employee_number ILIKE '%'||$2||'%'))
		  AND ($3 = '' OR status = $3)
		ORDER BY last_name, first_name
		LIMIT $4 OFFSET $5`,
		p.OrgID, p.Search, p.Status, p.Limit, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Worker
	for rows.Next() {
		wk, err := scanWorker(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, wk)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	err = s.pool.QueryRow(ctx, `
		SELECT count(*) FROM workers
		WHERE org_id = $1
		  AND ($2 = '' OR (first_name ILIKE '%'||$2||'%' OR last_name ILIKE '%'||$2||'%'
		                   OR work_email ILIKE '%'||$2||'%' OR employee_number ILIKE '%'||$2||'%'))
		  AND ($3 = '' OR status = $3)`,
		p.OrgID, p.Search, p.Status).Scan(&total)
	return out, total, err
}

// Update mutates editable fields of a worker within its org.
func (s *Store) Update(ctx context.Context, wk Worker) (Worker, error) {
	updated, err := scanWorker(s.pool.QueryRow(ctx, `
		UPDATE workers SET
			first_name = $3, last_name = $4, preferred_name = $5,
			work_email = $6, personal_email = $7, phone = $8,
			date_of_birth = $9, hire_date = $10, status = $11,
			address_line1 = $12, address_line2 = $13, city = $14, region = $15,
			postal_code = $16, country = $17,
			gender = $18, ethnicity = $19, marital_status = $20,
			work_auth_type = $21, work_auth_expiry = $22, i9_verified = $23, i9_verified_on = $24,
			updated_at = now()
		WHERE org_id = $1 AND id = $2
		RETURNING `+workerCols,
		wk.OrgID, wk.ID, wk.FirstName, wk.LastName, wk.PreferredName,
		wk.WorkEmail, wk.PersonalEmail, wk.Phone, wk.DateOfBirth, wk.HireDate, wk.Status,
		wk.AddressLine1, wk.AddressLine2, wk.City, wk.Region, wk.PostalCode, wk.Country,
		wk.Gender, wk.Ethnicity, wk.MaritalStatus,
		wk.WorkAuthType, wk.WorkAuthExpiry, wk.I9Verified, wk.I9VerifiedOn))
	if errors.Is(err, pgx.ErrNoRows) {
		return Worker{}, ErrNotFound
	}
	return updated, err
}

// LifecycleEvent records an employment event.
func (s *Store) LifecycleEvent(ctx context.Context, tx pgx.Tx, orgID, workerID uuid.UUID, eventType string, effectiveDate time.Time, reason string, createdBy *uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO lifecycle_events (org_id, worker_id, type, effective_date, reason, created_by)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		orgID, workerID, eventType, effectiveDate, reason, createdBy)
	return err
}

// SetStatusTx updates a worker's status within a transaction.
func (s *Store) SetStatusTx(ctx context.Context, tx pgx.Tx, orgID, id uuid.UUID, status string) error {
	_, err := tx.Exec(ctx,
		`UPDATE workers SET status=$3, updated_at=now() WHERE org_id=$1 AND id=$2`,
		orgID, id, status)
	return err
}

// CloseOpenAssignmentsTx ends any open assignments for a worker as of endDate.
// Used on termination. (Reads worker_assignments — owned by orgstructure — but
// termination is a worker-centric operation, so it lives with the worker.)
func (s *Store) CloseOpenAssignmentsTx(ctx context.Context, tx pgx.Tx, orgID, workerID uuid.UUID, endDate time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE worker_assignments SET end_date=$3
		WHERE org_id=$1 AND worker_id=$2 AND end_date IS NULL`,
		orgID, workerID, endDate)
	return err
}

// Event is a lifecycle event summary for the profile timeline.
type Event struct {
	ID            uuid.UUID `json:"id"`
	Type          string    `json:"type"`
	EffectiveDate time.Time `json:"effective_date"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListEvents returns a worker's lifecycle events, newest first.
func (s *Store) ListEvents(ctx context.Context, orgID, workerID uuid.UUID) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, effective_date, reason, created_at
		FROM lifecycle_events
		WHERE org_id=$1 AND worker_id=$2
		ORDER BY effective_date DESC, created_at DESC`, orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.EffectiveDate, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Profile is a worker enriched with their current assignment context.
type Profile struct {
	Worker
	PositionTitle  *string    `json:"position_title,omitempty"`
	DepartmentName *string    `json:"department_name,omitempty"`
	LocationName   *string    `json:"location_name,omitempty"`
	ManagerID      *uuid.UUID `json:"manager_id,omitempty"`
	ManagerName    *string    `json:"manager_name,omitempty"`
}

// GetProfile returns a worker plus their current (open primary) position,
// department, location, and manager — the data behind the profile page.
func (s *Store) GetProfile(ctx context.Context, orgID, id uuid.UUID) (Profile, error) {
	var p Profile
	err := s.pool.QueryRow(ctx, `
		SELECT `+prefixed("w", workerCols)+`,
			pos.title, dept.name, loc.name, a.manager_id,
			CASE WHEN mgr.id IS NULL THEN NULL
			     ELSE mgr.first_name || ' ' || mgr.last_name END
		FROM workers w
		LEFT JOIN worker_assignments a
			ON a.worker_id = w.id AND a.end_date IS NULL AND a.is_primary
		LEFT JOIN positions pos   ON pos.id = a.position_id
		LEFT JOIN departments dept ON dept.id = pos.department_id
		LEFT JOIN locations loc   ON loc.id = pos.location_id
		LEFT JOIN workers mgr     ON mgr.id = a.manager_id
		WHERE w.org_id = $1 AND w.id = $2`, orgID, id).
		Scan(&p.ID, &p.OrgID, &p.EmployeeNumber, &p.FirstName, &p.LastName,
			&p.PreferredName, &p.WorkEmail, &p.PersonalEmail, &p.Phone,
			&p.DateOfBirth, &p.HireDate, &p.Status,
			&p.AddressLine1, &p.AddressLine2, &p.City, &p.Region, &p.PostalCode, &p.Country,
			&p.Gender, &p.Ethnicity, &p.MaritalStatus,
			&p.WorkAuthType, &p.WorkAuthExpiry, &p.I9Verified, &p.I9VerifiedOn,
			&p.CreatedAt, &p.UpdatedAt,
			&p.PositionTitle, &p.DepartmentName, &p.LocationName, &p.ManagerID, &p.ManagerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	return p, err
}
