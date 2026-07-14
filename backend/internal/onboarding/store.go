// Package onboarding implements onboarding/offboarding checklists: reusable
// templates instantiated into per-worker plans with assignable, due-dated tasks.
package onboarding

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Errors returned by the store/service.
var (
	// ErrNotFound is returned when an entity does not exist in the org.
	ErrNotFound = errors.New("not found")
	// ErrForbidden is returned when a caller may not act on a task.
	ErrForbidden = errors.New("not authorized for this task")
)

// Template is a reusable checklist.
type Template struct {
	ID          uuid.UUID      `json:"id"`
	OrgID       uuid.UUID      `json:"org_id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Description string         `json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	Tasks       []TemplateTask `json:"tasks"`
}

// TemplateTask is a task within a template.
type TemplateTask struct {
	ID          uuid.UUID `json:"id"`
	TemplateID  uuid.UUID `json:"template_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Assignee    string    `json:"assignee"` // new_hire | manager | hr
	OffsetDays  int       `json:"offset_days"`
	SortOrder   int       `json:"sort_order"`
}

// Plan is a checklist instantiated for a worker.
type Plan struct {
	ID         uuid.UUID  `json:"id"`
	OrgID      uuid.UUID  `json:"org_id"`
	WorkerID   uuid.UUID  `json:"worker_id"`
	WorkerName string     `json:"worker_name,omitempty"`
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	StartDate  time.Time  `json:"start_date"`
	CreatedAt  time.Time  `json:"created_at"`
	TotalTasks int        `json:"total_tasks"`
	DoneTasks  int        `json:"done_tasks"`
}

// Task is a task within a plan.
type Task struct {
	ID               uuid.UUID  `json:"id"`
	PlanID           uuid.UUID  `json:"plan_id"`
	PlanName         string     `json:"plan_name,omitempty"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	AssigneeWorkerID *uuid.UUID `json:"assignee_worker_id,omitempty"`
	AssigneeName     *string    `json:"assignee_name,omitempty"`
	DueDate          *time.Time `json:"due_date,omitempty"`
	Status           string     `json:"status"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	SortOrder        int        `json:"sort_order"`
}

// Store provides org-scoped data access.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds an onboarding Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the pool for transactional services.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// --- Templates ---

// CreateTemplateTx inserts a template within a transaction.
func (s *Store) CreateTemplateTx(ctx context.Context, tx pgx.Tx, t Template) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO checklist_templates (org_id, name, type, description)
		VALUES ($1,$2,$3,$4) RETURNING id`, t.OrgID, t.Name, t.Type, t.Description).Scan(&id)
	return id, err
}

// AddTemplateTaskTx inserts a template task within a transaction.
func (s *Store) AddTemplateTaskTx(ctx context.Context, tx pgx.Tx, templateID uuid.UUID, t TemplateTask) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO checklist_template_tasks (template_id, title, description, assignee, offset_days, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		templateID, t.Title, t.Description, t.Assignee, t.OffsetDays, t.SortOrder)
	return err
}

// DeleteTemplateTasksTx removes all tasks from a template (for replace-on-update).
func (s *Store) DeleteTemplateTasksTx(ctx context.Context, tx pgx.Tx, templateID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM checklist_template_tasks WHERE template_id=$1`, templateID)
	return err
}

// UpdateTemplateTx edits a template's name/type/description within a transaction.
func (s *Store) UpdateTemplateTx(ctx context.Context, tx pgx.Tx, orgID, id uuid.UUID, name, typ, desc string) error {
	tag, err := tx.Exec(ctx, `UPDATE checklist_templates SET name=$3, type=$4, description=$5 WHERE org_id=$1 AND id=$2`,
		orgID, id, name, typ, desc)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTemplates returns all templates in an org with their tasks.
func (s *Store) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]Template, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, org_id, name, type, description, created_at
		FROM checklist_templates WHERE org_id=$1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []Template
	byID := map[uuid.UUID]int{}
	for rows.Next() {
		var t Template
		if err := rows.Scan(&t.ID, &t.OrgID, &t.Name, &t.Type, &t.Description, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Tasks = []TemplateTask{}
		byID[t.ID] = len(templates)
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return templates, nil
	}

	taskRows, err := s.pool.Query(ctx, `
		SELECT tt.id, tt.template_id, tt.title, tt.description, tt.assignee, tt.offset_days, tt.sort_order
		FROM checklist_template_tasks tt
		JOIN checklist_templates t ON t.id = tt.template_id
		WHERE t.org_id=$1
		ORDER BY tt.sort_order`, orgID)
	if err != nil {
		return nil, err
	}
	defer taskRows.Close()
	for taskRows.Next() {
		var tt TemplateTask
		if err := taskRows.Scan(&tt.ID, &tt.TemplateID, &tt.Title, &tt.Description, &tt.Assignee, &tt.OffsetDays, &tt.SortOrder); err != nil {
			return nil, err
		}
		if idx, ok := byID[tt.TemplateID]; ok {
			templates[idx].Tasks = append(templates[idx].Tasks, tt)
		}
	}
	return templates, taskRows.Err()
}

// GetTemplateTasks returns a template's tasks (used at instantiation).
func (s *Store) GetTemplateTasks(ctx context.Context, orgID, templateID uuid.UUID) (Template, []TemplateTask, error) {
	var t Template
	err := s.pool.QueryRow(ctx, `
		SELECT id, org_id, name, type, description, created_at
		FROM checklist_templates WHERE org_id=$1 AND id=$2`, orgID, templateID).
		Scan(&t.ID, &t.OrgID, &t.Name, &t.Type, &t.Description, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Template{}, nil, ErrNotFound
	}
	if err != nil {
		return Template{}, nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, template_id, title, description, assignee, offset_days, sort_order
		FROM checklist_template_tasks WHERE template_id=$1 ORDER BY sort_order`, templateID)
	if err != nil {
		return Template{}, nil, err
	}
	defer rows.Close()
	var tasks []TemplateTask
	for rows.Next() {
		var tt TemplateTask
		if err := rows.Scan(&tt.ID, &tt.TemplateID, &tt.Title, &tt.Description, &tt.Assignee, &tt.OffsetDays, &tt.SortOrder); err != nil {
			return Template{}, nil, err
		}
		tasks = append(tasks, tt)
	}
	return t, tasks, rows.Err()
}

// DeleteTemplate removes a template.
func (s *Store) DeleteTemplate(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM checklist_templates WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Plans & tasks ---

// CreatePlanTx inserts a plan within a transaction.
func (s *Store) CreatePlanTx(ctx context.Context, tx pgx.Tx, p Plan) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRow(ctx, `
		INSERT INTO checklist_plans (org_id, worker_id, template_id, name, type, start_date)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		p.OrgID, p.WorkerID, p.TemplateID, p.Name, p.Type, p.StartDate).Scan(&id)
	return id, err
}

// CreateTaskTx inserts a plan task within a transaction.
func (s *Store) CreateTaskTx(ctx context.Context, tx pgx.Tx, orgID, planID uuid.UUID, t Task) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO checklist_tasks (org_id, plan_id, title, description, assignee_worker_id, due_date, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		orgID, planID, t.Title, t.Description, t.AssigneeWorkerID, t.DueDate, t.SortOrder)
	return err
}

// ListPlans returns plans with task progress, optionally filtered by worker.
func (s *Store) ListPlans(ctx context.Context, orgID uuid.UUID, workerID *uuid.UUID) ([]Plan, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.org_id, p.worker_id, w.first_name || ' ' || w.last_name,
			p.template_id, p.name, p.type, p.status, p.start_date, p.created_at,
			count(t.id), count(t.id) FILTER (WHERE t.status='done')
		FROM checklist_plans p
		JOIN workers w ON w.id = p.worker_id
		LEFT JOIN checklist_tasks t ON t.plan_id = p.id
		WHERE p.org_id=$1 AND ($2::uuid IS NULL OR p.worker_id=$2)
		GROUP BY p.id, w.id
		ORDER BY p.created_at DESC`, orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.OrgID, &p.WorkerID, &p.WorkerName, &p.TemplateID,
			&p.Name, &p.Type, &p.Status, &p.StartDate, &p.CreatedAt, &p.TotalTasks, &p.DoneTasks); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPlan returns a plan by id with worker name.
func (s *Store) GetPlan(ctx context.Context, orgID, id uuid.UUID) (Plan, error) {
	var p Plan
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, p.org_id, p.worker_id, w.first_name || ' ' || w.last_name,
			p.template_id, p.name, p.type, p.status, p.start_date, p.created_at
		FROM checklist_plans p JOIN workers w ON w.id = p.worker_id
		WHERE p.org_id=$1 AND p.id=$2`, orgID, id).
		Scan(&p.ID, &p.OrgID, &p.WorkerID, &p.WorkerName, &p.TemplateID,
			&p.Name, &p.Type, &p.Status, &p.StartDate, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	return p, err
}

// PlanTasks returns a plan's tasks with assignee names.
func (s *Store) PlanTasks(ctx context.Context, orgID, planID uuid.UUID) ([]Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.plan_id, t.title, t.description, t.assignee_worker_id,
			CASE WHEN a.id IS NULL THEN NULL ELSE a.first_name || ' ' || a.last_name END,
			t.due_date, t.status, t.completed_at, t.sort_order
		FROM checklist_tasks t
		LEFT JOIN workers a ON a.id = t.assignee_worker_id
		WHERE t.org_id=$1 AND t.plan_id=$2
		ORDER BY t.sort_order`, orgID, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTasks(rows)
}

// SetTaskStatus updates a task's status and completed_at.
func (s *Store) SetTaskStatus(ctx context.Context, orgID, id uuid.UUID, status string) (uuid.UUID, error) {
	var planID uuid.UUID
	var completed any
	if status == "done" {
		completed = time.Now()
	}
	err := s.pool.QueryRow(ctx, `
		UPDATE checklist_tasks SET status=$3, completed_at=$4 WHERE org_id=$1 AND id=$2 RETURNING plan_id`,
		orgID, id, status, completed).Scan(&planID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return planID, err
}

// RecomputePlanStatus sets a plan to completed if all its tasks are done, else active.
func (s *Store) RecomputePlanStatus(ctx context.Context, orgID, planID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE checklist_plans SET status = CASE
			WHEN NOT EXISTS (SELECT 1 FROM checklist_tasks WHERE plan_id=$2 AND status<>'done')
				 AND EXISTS (SELECT 1 FROM checklist_tasks WHERE plan_id=$2)
			THEN 'completed' ELSE 'active' END
		WHERE org_id=$1 AND id=$2 AND status <> 'cancelled'`, orgID, planID)
	return err
}

// MyTasks returns pending tasks assigned to a worker across plans.
func (s *Store) MyTasks(ctx context.Context, orgID, workerID uuid.UUID) ([]Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.id, t.plan_id, t.title, t.description, t.assignee_worker_id, NULL,
			t.due_date, t.status, t.completed_at, t.sort_order, p.name
		FROM checklist_tasks t
		JOIN checklist_plans p ON p.id = t.plan_id
		WHERE t.org_id=$1 AND t.assignee_worker_id=$2 AND t.status='pending'
		ORDER BY t.due_date NULLS LAST`, orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		var assigneeName *string
		if err := rows.Scan(&t.ID, &t.PlanID, &t.Title, &t.Description, &t.AssigneeWorkerID, &assigneeName,
			&t.DueDate, &t.Status, &t.CompletedAt, &t.SortOrder, &t.PlanName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
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

// TaskAssignee returns a task's assignee worker id (nil if unassigned).
func (s *Store) TaskAssignee(ctx context.Context, orgID, taskID uuid.UUID) (*uuid.UUID, error) {
	var assignee *uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT assignee_worker_id FROM checklist_tasks WHERE org_id=$1 AND id=$2`, orgID, taskID).Scan(&assignee)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return assignee, err
}

// CurrentManager returns the manager on a worker's open primary assignment.
func (s *Store) CurrentManager(ctx context.Context, orgID, workerID uuid.UUID) (*uuid.UUID, error) {
	var mgr *uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT manager_id FROM worker_assignments
		WHERE org_id=$1 AND worker_id=$2 AND end_date IS NULL AND is_primary LIMIT 1`, orgID, workerID).Scan(&mgr)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return mgr, err
}

func scanTasks(rows pgx.Rows) ([]Task, error) {
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.PlanID, &t.Title, &t.Description, &t.AssigneeWorkerID,
			&t.AssigneeName, &t.DueDate, &t.Status, &t.CompletedAt, &t.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
