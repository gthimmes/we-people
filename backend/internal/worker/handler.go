package worker

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// Handler exposes worker HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds a worker Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts worker routes under an authenticated router.
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission("worker:read")).Get("/", h.list)
	r.With(auth.RequirePermission("worker:read")).Get("/{id}", h.get)
	r.With(auth.RequirePermission("worker:write")).Post("/", h.create)
	r.With(auth.RequirePermission("worker:write")).Put("/{id}", h.update)
}

type workerRequest struct {
	EmployeeNumber string  `json:"employee_number"`
	FirstName      string  `json:"first_name"`
	LastName       string  `json:"last_name"`
	PreferredName  string  `json:"preferred_name"`
	WorkEmail      string  `json:"work_email"`
	PersonalEmail  string  `json:"personal_email"`
	Phone          string  `json:"phone"`
	DateOfBirth    *string `json:"date_of_birth"`
	HireDate       *string `json:"hire_date"`
	Status         string  `json:"status"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req workerRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if fields := validateWorker(req); len(fields) > 0 {
		httpx.ValidationError(w, fields)
		return
	}
	dob, dobErr := parseDate(req.DateOfBirth)
	hire, hireErr := parseDate(req.HireDate)
	if dobErr != nil || hireErr != nil {
		httpx.ValidationError(w, map[string]string{"date": "dates must be YYYY-MM-DD"})
		return
	}
	created, err := h.svc.Create(r.Context(), p.OrgID, p.UserID, CreateInput{
		EmployeeNumber: req.EmployeeNumber,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		PreferredName:  req.PreferredName,
		WorkEmail:      req.WorkEmail,
		PersonalEmail:  req.PersonalEmail,
		Phone:          req.Phone,
		DateOfBirth:    dob,
		HireDate:       hire,
		Status:         req.Status,
	})
	if err != nil {
		if strings.Contains(err.Error(), "SQLSTATE 23505") {
			httpx.Error(w, http.StatusConflict, "duplicate", "employee number already exists")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not create worker")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	workers, total, err := h.svc.List(r.Context(), ListParams{
		OrgID:  p.OrgID,
		Search: q.Get("search"),
		Status: q.Get("status"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list workers")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"data": workers,
		"meta": map[string]any{"total": total, "limit": limit, "offset": offset},
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	wk, err := h.svc.Get(r.Context(), p.OrgID, id)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "worker not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not fetch worker")
		return
	}
	httpx.JSON(w, http.StatusOK, wk)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	var req workerRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if fields := validateWorker(req); len(fields) > 0 {
		httpx.ValidationError(w, fields)
		return
	}
	dob, dobErr := parseDate(req.DateOfBirth)
	hire, hireErr := parseDate(req.HireDate)
	if dobErr != nil || hireErr != nil {
		httpx.ValidationError(w, map[string]string{"date": "dates must be YYYY-MM-DD"})
		return
	}
	updated, err := h.svc.Update(r.Context(), p.OrgID, p.UserID, id, UpdateInput{
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		PreferredName: req.PreferredName,
		WorkEmail:     req.WorkEmail,
		PersonalEmail: req.PersonalEmail,
		Phone:         req.Phone,
		DateOfBirth:   dob,
		HireDate:      hire,
		Status:        req.Status,
	})
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "worker not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not update worker")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func validateWorker(req workerRequest) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(req.FirstName) == "" {
		fields["first_name"] = "first name is required"
	}
	if strings.TrimSpace(req.LastName) == "" {
		fields["last_name"] = "last name is required"
	}
	if strings.TrimSpace(req.EmployeeNumber) == "" {
		fields["employee_number"] = "employee number is required"
	}
	return fields
}

// parseDate parses an optional YYYY-MM-DD string into a *time.Time.
func parseDate(s *string) (*time.Time, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(*s))
	if err != nil {
		return nil, err
	}
	return &t, nil
}
