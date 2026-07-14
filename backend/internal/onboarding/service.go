package onboarding

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/audit"
)

// Notifier delivers a notification to the user linked to a worker (best-effort).
type Notifier interface {
	NotifyWorker(ctx context.Context, orgID, workerID uuid.UUID, typ, title, body, link string)
}

// Service implements onboarding business logic.
type Service struct {
	store    *Store
	audit    *audit.Logger
	notifier Notifier
}

// NewService builds the onboarding service.
func NewService(store *Store, auditLog *audit.Logger) *Service {
	return &Service{store: store, audit: auditLog}
}

// SetNotifier wires the notification sink (optional).
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// TemplateInput is the payload for creating/updating a template.
type TemplateInput struct {
	Name        string
	Type        string
	Description string
	Tasks       []TemplateTask
}

// CreateTemplate creates a template with its tasks.
func (s *Service) CreateTemplate(ctx context.Context, orgID, actor uuid.UUID, in TemplateInput) (Template, error) {
	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)

	id, err := s.store.CreateTemplateTx(ctx, tx, Template{OrgID: orgID, Name: in.Name, Type: in.Type, Description: in.Description})
	if err != nil {
		return Template{}, err
	}
	for i, t := range in.Tasks {
		t.SortOrder = i
		if err := s.store.AddTemplateTaskTx(ctx, tx, id, t); err != nil {
			return Template{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "checklist_template.create", EntityType: "checklist_template", EntityID: &id})
	tmpl, tasks, err := s.store.GetTemplateTasks(ctx, orgID, id)
	tmpl.Tasks = tasks
	return tmpl, err
}

// UpdateTemplate edits a template and replaces its tasks.
func (s *Service) UpdateTemplate(ctx context.Context, orgID, actor, id uuid.UUID, in TemplateInput) (Template, error) {
	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return Template{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.store.UpdateTemplateTx(ctx, tx, orgID, id, in.Name, in.Type, in.Description); err != nil {
		return Template{}, err
	}
	if err := s.store.DeleteTemplateTasksTx(ctx, tx, id); err != nil {
		return Template{}, err
	}
	for i, t := range in.Tasks {
		t.SortOrder = i
		if err := s.store.AddTemplateTaskTx(ctx, tx, id, t); err != nil {
			return Template{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Template{}, err
	}
	s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "checklist_template.update", EntityType: "checklist_template", EntityID: &id})
	tmpl, tasks, err := s.store.GetTemplateTasks(ctx, orgID, id)
	tmpl.Tasks = tasks
	return tmpl, err
}

// ListTemplates returns templates (non-nil slice).
func (s *Service) ListTemplates(ctx context.Context, orgID uuid.UUID) ([]Template, error) {
	t, err := s.store.ListTemplates(ctx, orgID)
	if t == nil {
		t = []Template{}
	}
	return t, err
}

// DeleteTemplate removes a template.
func (s *Service) DeleteTemplate(ctx context.Context, orgID, actor, id uuid.UUID) error {
	if err := s.store.DeleteTemplate(ctx, orgID, id); err != nil {
		return err
	}
	s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "checklist_template.delete", EntityType: "checklist_template", EntityID: &id})
	return nil
}

// CreatePlan instantiates a template into a per-worker plan, resolving each
// task's assignee (new hire / manager / HR) and due date, then notifies the
// new hire and their manager.
func (s *Service) CreatePlan(ctx context.Context, orgID, actor, workerID, templateID uuid.UUID, startDate time.Time) (Plan, error) {
	tmpl, tasks, err := s.store.GetTemplateTasks(ctx, orgID, templateID)
	if err != nil {
		return Plan{}, err
	}
	manager, err := s.store.CurrentManager(ctx, orgID, workerID)
	if err != nil {
		return Plan{}, err
	}

	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return Plan{}, err
	}
	defer tx.Rollback(ctx)

	planID, err := s.store.CreatePlanTx(ctx, tx, Plan{
		OrgID: orgID, WorkerID: workerID, TemplateID: &templateID,
		Name: tmpl.Name, Type: tmpl.Type, StartDate: startDate,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("create plan: %w", err)
	}
	for i, tt := range tasks {
		var assignee *uuid.UUID
		switch tt.Assignee {
		case "new_hire":
			w := workerID
			assignee = &w
		case "manager":
			assignee = manager
		}
		var due *time.Time
		d := startDate.AddDate(0, 0, tt.OffsetDays)
		due = &d
		if err := s.store.CreateTaskTx(ctx, tx, orgID, planID, Task{
			Title: tt.Title, Description: tt.Description, AssigneeWorkerID: assignee, DueDate: due, SortOrder: i,
		}); err != nil {
			return Plan{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Plan{}, err
	}

	s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "checklist_plan.create", EntityType: "checklist_plan", EntityID: &planID})
	if s.notifier != nil {
		verb := "Onboarding"
		if tmpl.Type == "offboarding" {
			verb = "Offboarding"
		}
		s.notifier.NotifyWorker(ctx, orgID, workerID, "checklist.assigned", verb+" tasks assigned to you", tmpl.Name, "/onboarding")
		if manager != nil {
			s.notifier.NotifyWorker(ctx, orgID, *manager, "checklist.assigned", verb+" tasks for your report", tmpl.Name, "/onboarding")
		}
	}
	return s.store.GetPlan(ctx, orgID, planID)
}

// ListPlans returns plans with progress (non-nil slice).
func (s *Service) ListPlans(ctx context.Context, orgID uuid.UUID, workerID *uuid.UUID) ([]Plan, error) {
	p, err := s.store.ListPlans(ctx, orgID, workerID)
	if p == nil {
		p = []Plan{}
	}
	return p, err
}

// PlanWithTasks bundles a plan and its tasks.
type PlanWithTasks struct {
	Plan
	Tasks []Task `json:"tasks"`
}

// GetPlan returns a plan with its tasks.
func (s *Service) GetPlan(ctx context.Context, orgID, id uuid.UUID) (PlanWithTasks, error) {
	plan, err := s.store.GetPlan(ctx, orgID, id)
	if err != nil {
		return PlanWithTasks{}, err
	}
	tasks, err := s.store.PlanTasks(ctx, orgID, id)
	if err != nil {
		return PlanWithTasks{}, err
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return PlanWithTasks{Plan: plan, Tasks: tasks}, nil
}

// SetTaskStatus marks a task done or pending and recomputes the plan status.
// The caller must be the task's assignee or an admin.
func (s *Service) SetTaskStatus(ctx context.Context, orgID, actorUserID, taskID uuid.UUID, status string, adminOverride bool) error {
	if !adminOverride {
		assignee, err := s.store.TaskAssignee(ctx, orgID, taskID)
		if err != nil {
			return err
		}
		caller, err := s.store.WorkerIDForUser(ctx, orgID, actorUserID)
		if err != nil && err != ErrNotFound {
			return err
		}
		if assignee == nil || caller == nil || *assignee != *caller {
			return ErrForbidden
		}
	}
	planID, err := s.store.SetTaskStatus(ctx, orgID, taskID, status)
	if err != nil {
		return err
	}
	return s.store.RecomputePlanStatus(ctx, orgID, planID)
}

// MyTasks returns the caller's pending checklist tasks (non-nil slice).
func (s *Service) MyTasks(ctx context.Context, orgID, userID uuid.UUID) ([]Task, error) {
	worker, err := s.store.WorkerIDForUser(ctx, orgID, userID)
	if err != nil || worker == nil {
		return []Task{}, nil
	}
	t, err := s.store.MyTasks(ctx, orgID, *worker)
	if t == nil {
		t = []Task{}
	}
	return t, err
}
