package onboarding

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// Handler exposes onboarding HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds an onboarding Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts onboarding routes under an authenticated router.
func (h *Handler) Routes(r chi.Router) {
	read := auth.RequirePermission("worker:read")
	write := auth.RequirePermission("worker:write")

	r.With(read).Get("/checklist-templates", h.listTemplates)
	r.With(write).Post("/checklist-templates", h.createTemplate)
	r.With(write).Put("/checklist-templates/{id}", h.updateTemplate)
	r.With(write).Delete("/checklist-templates/{id}", h.deleteTemplate)
	r.With(read).Get("/checklist-plans", h.listPlans)
	r.With(write).Post("/checklist-plans", h.createPlan)
	r.With(read).Get("/checklist-plans/{id}", h.getPlan)
	r.Post("/checklist-tasks/{id}/status", h.setTaskStatus) // assignee or admin
	r.Get("/my-tasks", h.myTasks)
}

type templateTaskReq struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	OffsetDays  int    `json:"offset_days"`
}

type templateReq struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Tasks       []templateTaskReq `json:"tasks"`
}

func (r templateReq) toInput() (TemplateInput, map[string]string) {
	fields := map[string]string{}
	if strings.TrimSpace(r.Name) == "" {
		fields["name"] = "name is required"
	}
	typ := r.Type
	if typ == "" {
		typ = "onboarding"
	}
	if typ != "onboarding" && typ != "offboarding" {
		fields["type"] = "must be onboarding or offboarding"
	}
	tasks := make([]TemplateTask, 0, len(r.Tasks))
	for _, t := range r.Tasks {
		if strings.TrimSpace(t.Title) == "" {
			continue
		}
		assignee := t.Assignee
		if assignee == "" {
			assignee = "new_hire"
		}
		tasks = append(tasks, TemplateTask{Title: t.Title, Description: t.Description, Assignee: assignee, OffsetDays: t.OffsetDays})
	}
	return TemplateInput{Name: r.Name, Type: typ, Description: r.Description, Tasks: tasks}, fields
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req templateReq
	if !httpx.Decode(w, r, &req) {
		return
	}
	in, fields := req.toInput()
	if len(fields) > 0 {
		httpx.ValidationError(w, fields)
		return
	}
	t, err := h.svc.CreateTemplate(r.Context(), p.OrgID, p.UserID, in)
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			httpx.Error(w, http.StatusConflict, "duplicate", "a template with that name already exists")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not create template")
		return
	}
	httpx.JSON(w, http.StatusCreated, t)
}

func (h *Handler) updateTemplate(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid template id")
		return
	}
	var req templateReq
	if !httpx.Decode(w, r, &req) {
		return
	}
	in, fields := req.toInput()
	if len(fields) > 0 {
		httpx.ValidationError(w, fields)
		return
	}
	t, err := h.svc.UpdateTemplate(r.Context(), p.OrgID, p.UserID, id, in)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not update template")
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	t, err := h.svc.ListTemplates(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list templates")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": t})
}

func (h *Handler) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid template id")
		return
	}
	if err := h.svc.DeleteTemplate(r.Context(), p.OrgID, p.UserID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "template not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not delete template")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

type createPlanReq struct {
	WorkerID   string  `json:"worker_id"`
	TemplateID string  `json:"template_id"`
	StartDate  *string `json:"start_date"`
}

func (h *Handler) createPlan(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req createPlanReq
	if !httpx.Decode(w, r, &req) {
		return
	}
	workerID, err := uuid.Parse(req.WorkerID)
	if err != nil {
		httpx.ValidationError(w, map[string]string{"worker_id": "valid worker_id is required"})
		return
	}
	templateID, err := uuid.Parse(req.TemplateID)
	if err != nil {
		httpx.ValidationError(w, map[string]string{"template_id": "valid template_id is required"})
		return
	}
	start := time.Now()
	if req.StartDate != nil && strings.TrimSpace(*req.StartDate) != "" {
		start, err = time.Parse("2006-01-02", strings.TrimSpace(*req.StartDate))
		if err != nil {
			httpx.ValidationError(w, map[string]string{"start_date": "must be YYYY-MM-DD"})
			return
		}
	}
	plan, err := h.svc.CreatePlan(r.Context(), p.OrgID, p.UserID, workerID, templateID, start)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not create plan")
		return
	}
	httpx.JSON(w, http.StatusCreated, plan)
}

func (h *Handler) listPlans(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var workerID *uuid.UUID
	if q := r.URL.Query().Get("worker_id"); q != "" {
		id, err := uuid.Parse(q)
		if err != nil {
			httpx.ValidationError(w, map[string]string{"worker_id": "invalid UUID"})
			return
		}
		workerID = &id
	}
	plans, err := h.svc.ListPlans(r.Context(), p.OrgID, workerID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list plans")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": plans})
}

func (h *Handler) getPlan(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid plan id")
		return
	}
	plan, err := h.svc.GetPlan(r.Context(), p.OrgID, id)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "plan not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not fetch plan")
		return
	}
	httpx.JSON(w, http.StatusOK, plan)
}

type taskStatusReq struct {
	Status string `json:"status"`
}

func (h *Handler) setTaskStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid task id")
		return
	}
	var req taskStatusReq
	if !httpx.Decode(w, r, &req) {
		return
	}
	if req.Status != "done" && req.Status != "pending" {
		httpx.ValidationError(w, map[string]string{"status": "must be done or pending"})
		return
	}
	err = h.svc.SetTaskStatus(r.Context(), p.OrgID, p.UserID, id, req.Status, p.Can("worker:write"))
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "task not found")
	case errors.Is(err, ErrForbidden):
		httpx.Error(w, http.StatusForbidden, "forbidden", "this task isn't assigned to you")
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not update task")
	default:
		httpx.JSON(w, http.StatusNoContent, nil)
	}
}

func (h *Handler) myTasks(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	tasks, err := h.svc.MyTasks(r.Context(), p.OrgID, p.UserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not fetch tasks")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": tasks})
}
