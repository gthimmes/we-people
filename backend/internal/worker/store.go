// Package worker owns worker (employee) records and employment lifecycle events.
package worker

import (
	"context"
	"errors"
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
	work_email, personal_email, phone, date_of_birth, hire_date, status, created_at, updated_at`

func scanWorker(row pgx.Row) (Worker, error) {
	var wk Worker
	err := row.Scan(&wk.ID, &wk.OrgID, &wk.EmployeeNumber, &wk.FirstName, &wk.LastName,
		&wk.PreferredName, &wk.WorkEmail, &wk.PersonalEmail, &wk.Phone,
		&wk.DateOfBirth, &wk.HireDate, &wk.Status, &wk.CreatedAt, &wk.UpdatedAt)
	return wk, err
}

// Create inserts a worker within a transaction.
func (s *Store) Create(ctx context.Context, tx pgx.Tx, wk Worker) (Worker, error) {
	return scanWorker(tx.QueryRow(ctx, `
		INSERT INTO workers (org_id, employee_number, first_name, last_name, preferred_name,
			work_email, personal_email, phone, date_of_birth, hire_date, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING `+workerCols,
		wk.OrgID, wk.EmployeeNumber, wk.FirstName, wk.LastName, wk.PreferredName,
		wk.WorkEmail, wk.PersonalEmail, wk.Phone, wk.DateOfBirth, wk.HireDate, wk.Status))
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
			date_of_birth = $9, hire_date = $10, status = $11, updated_at = now()
		WHERE org_id = $1 AND id = $2
		RETURNING `+workerCols,
		wk.OrgID, wk.ID, wk.FirstName, wk.LastName, wk.PreferredName,
		wk.WorkEmail, wk.PersonalEmail, wk.Phone, wk.DateOfBirth, wk.HireDate, wk.Status))
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
