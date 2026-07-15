package timeoff

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

// Handler exposes time-off HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds a time-off Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts time-off routes. Any authenticated user may request their own
// time off and read their own balances/requests.
func (h *Handler) Routes(r chi.Router) {
	admin := auth.RequirePermission("org:write")
	r.Get("/leave-types", h.listLeaveTypes)
	r.With(admin).Post("/leave-types", h.createLeaveType)
	r.With(admin).Put("/leave-types/{id}", h.updateLeaveType)
	r.With(admin).Delete("/leave-types/{id}", h.deleteLeaveType)
	r.With(admin).Post("/accruals/run", h.runAccrual)
	r.With(admin).Post("/accruals/carryover", h.runCarryover)
	r.Get("/balances", h.balances)     // ?worker_id= (defaults to caller)
	r.Get("/requests", h.listRequests) // ?worker_id= (defaults to caller)
	r.Post("/requests", h.createRequest)
	r.Delete("/requests/{id}", h.cancelRequest) // withdraw a pending request
}

func (h *Handler) updateLeaveType(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid leave type id")
		return
	}
	var req leaveTypeRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	lt, err := h.svc.UpdateLeaveType(r.Context(), p.OrgID, id, req.toLeaveType())
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "leave type not found")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			httpx.Error(w, http.StatusConflict, "duplicate", "a leave type with that name already exists")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not update leave type")
		return
	}
	httpx.JSON(w, http.StatusOK, lt)
}

func (h *Handler) deleteLeaveType(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid leave type id")
		return
	}
	err = h.svc.DeleteLeaveType(r.Context(), p.OrgID, id)
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "leave type not found")
	case err != nil && strings.Contains(err.Error(), "SQLSTATE 23503"):
		httpx.Error(w, http.StatusConflict, "in_use", "this leave type is used by existing requests and cannot be deleted")
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not delete leave type")
	default:
		httpx.JSON(w, http.StatusNoContent, nil)
	}
}

func (h *Handler) cancelRequest(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid request id")
		return
	}
	err = h.svc.Cancel(r.Context(), p.OrgID, p.UserID, id, p.Can("org:write"))
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "request not found")
	case errors.Is(err, ErrNotOwner):
		httpx.Error(w, http.StatusForbidden, "forbidden", "you can only cancel your own requests")
	case errors.Is(err, ErrNotCancellable):
		httpx.Error(w, http.StatusConflict, "not_cancellable", "only pending requests can be cancelled")
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not cancel request")
	default:
		httpx.JSON(w, http.StatusNoContent, nil)
	}
}

type leaveTypeRequest struct {
	Name               string   `json:"name"`
	IsPaid             *bool    `json:"is_paid"`
	AccrualEnabled     bool     `json:"accrual_enabled"`
	AccrualAnnualHours float64  `json:"accrual_annual_hours"`
	MaxBalanceHours    float64  `json:"max_balance_hours"`
	CarryoverMaxHours  *float64 `json:"carryover_max_hours"`
}

func (r leaveTypeRequest) toLeaveType() LeaveType {
	isPaid := true
	if r.IsPaid != nil {
		isPaid = *r.IsPaid
	}
	return LeaveType{
		Name: r.Name, IsPaid: isPaid,
		AccrualEnabled: r.AccrualEnabled, AccrualAnnualHours: r.AccrualAnnualHours,
		MaxBalanceHours: r.MaxBalanceHours, CarryoverMaxHours: r.CarryoverMaxHours,
	}
}

func (h *Handler) createLeaveType(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req leaveTypeRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	lt, err := h.svc.CreateLeaveType(r.Context(), p.OrgID, req.toLeaveType())
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			httpx.Error(w, http.StatusConflict, "duplicate", "a leave type with that name already exists")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not create leave type")
		return
	}
	httpx.JSON(w, http.StatusCreated, lt)
}

type runAccrualRequest struct {
	Period string `json:"period"` // YYYY-MM; defaults to current month
}

func (h *Handler) runAccrual(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req runAccrualRequest
	if r.ContentLength > 0 && !httpx.Decode(w, r, &req) {
		return
	}
	period := req.Period
	if period == "" {
		period = time.Now().Format("2006-01")
	}
	res, err := h.svc.RunAccrual(r.Context(), p.OrgID, period)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not run accrual")
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

type runCarryoverRequest struct {
	Year int `json:"year"`
}

func (h *Handler) runCarryover(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req runCarryoverRequest
	if r.ContentLength > 0 && !httpx.Decode(w, r, &req) {
		return
	}
	year := req.Year
	if year == 0 {
		year = time.Now().Year()
	}
	res, err := h.svc.RunCarryover(r.Context(), p.OrgID, year)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not run carryover")
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) listLeaveTypes(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	lt, err := h.svc.ListLeaveTypes(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list leave types")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": lt})
}

func (h *Handler) balances(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	workerID, ok := h.resolveWorker(w, r)
	if !ok {
		return
	}
	b, err := h.svc.Balances(r.Context(), p.OrgID, workerID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list balances")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": b})
}

func (h *Handler) listRequests(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	workerID, ok := h.resolveWorker(w, r)
	if !ok {
		return
	}
	reqs, err := h.svc.ListForWorker(r.Context(), p.OrgID, workerID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list requests")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": reqs})
}

type createRequest struct {
	WorkerID    *string `json:"worker_id"`
	LeaveTypeID string  `json:"leave_type_id"`
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	Hours       float64 `json:"hours"`
	Reason      string  `json:"reason"`
}

func (h *Handler) createRequest(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req createRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	fields := map[string]string{}
	leaveTypeID, err := uuid.Parse(req.LeaveTypeID)
	if err != nil {
		fields["leave_type_id"] = "valid leave_type_id is required"
	}
	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		fields["start_date"] = "start_date must be YYYY-MM-DD"
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		fields["end_date"] = "end_date must be YYYY-MM-DD"
	}
	if req.Hours <= 0 {
		fields["hours"] = "hours must be greater than 0"
	}
	if err == nil && end.Before(start) {
		fields["end_date"] = "end_date cannot be before start_date"
	}
	if len(fields) > 0 {
		httpx.ValidationError(w, fields)
		return
	}

	explicit, ok := optionalUUID(w, req.WorkerID)
	if !ok {
		return
	}
	workerID, err := h.svc.ResolveWorker(r.Context(), p.OrgID, p.UserID, explicit)
	if errors.Is(err, ErrNoWorker) {
		httpx.Error(w, http.StatusBadRequest, "no_worker", "no worker linked to your user; specify worker_id")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not resolve worker")
		return
	}

	created, err := h.svc.Create(r.Context(), p.OrgID, p.UserID, CreateInput{
		WorkerID: workerID, LeaveTypeID: leaveTypeID,
		StartDate: start, EndDate: end, Hours: req.Hours, Reason: req.Reason,
	})
	if errors.Is(err, ErrInvalidLeaveType) {
		httpx.ValidationError(w, map[string]string{"leave_type_id": "unknown leave type"})
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not create request")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

// resolveWorker returns the worker to operate on: an explicit ?worker_id or the
// caller's linked worker.
func (h *Handler) resolveWorker(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	p := auth.PrincipalFrom(r.Context())
	explicit, ok := optionalUUID(w, ptrOrNil(r.URL.Query().Get("worker_id")))
	if !ok {
		return uuid.Nil, false
	}
	workerID, err := h.svc.ResolveWorker(r.Context(), p.OrgID, p.UserID, explicit)
	if errors.Is(err, ErrNoWorker) {
		httpx.Error(w, http.StatusBadRequest, "no_worker", "no worker linked to your user; specify worker_id")
		return uuid.Nil, false
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not resolve worker")
		return uuid.Nil, false
	}
	return workerID, true
}

func optionalUUID(w http.ResponseWriter, s *string) (*uuid.UUID, bool) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, true
	}
	id, err := uuid.Parse(strings.TrimSpace(*s))
	if err != nil {
		httpx.ValidationError(w, map[string]string{"worker_id": "invalid UUID"})
		return nil, false
	}
	return &id, true
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
