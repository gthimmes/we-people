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
	read := auth.RequirePermission("worker:read")
	write := auth.RequirePermission("worker:write")

	r.With(read).Get("/", h.list)
	r.With(read).Get("/{id}", h.get)
	r.With(read).Get("/{id}/profile", h.profile)
	r.With(read).Get("/{id}/events", h.events)
	r.With(read).Get("/{id}/emergency-contacts", h.listContacts)
	r.With(write).Post("/", h.create)
	r.With(write).Put("/{id}", h.update)
	r.With(write).Delete("/{id}", h.deleteWorker)
	r.With(write).Post("/{id}/terminate", h.terminate)
	r.With(write).Post("/{id}/emergency-contacts", h.addContact)
	r.With(write).Put("/{id}/emergency-contacts/{contactId}", h.updateContact)
	r.With(write).Delete("/{id}/emergency-contacts/{contactId}", h.deleteContact)
}

func (h *Handler) deleteWorker(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	err = h.svc.Delete(r.Context(), p.OrgID, p.UserID, id)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "worker not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not delete worker")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
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
	// Home address
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	City         string `json:"city"`
	Region       string `json:"region"`
	PostalCode   string `json:"postal_code"`
	Country      string `json:"country"`
	// Demographics
	Gender        string `json:"gender"`
	Ethnicity     string `json:"ethnicity"`
	MaritalStatus string `json:"marital_status"`
	// Work eligibility / I-9
	WorkAuthType   string  `json:"work_auth_type"`
	WorkAuthExpiry *string `json:"work_auth_expiry"`
	I9Verified     bool    `json:"i9_verified"`
	I9VerifiedOn   *string `json:"i9_verified_on"`
}

// personal maps request fields to the shared PersonalFields struct. Date fields
// are parsed leniently — invalid dates are treated as unset.
func (req workerRequest) personal() PersonalFields {
	authExpiry, _ := parseDate(req.WorkAuthExpiry)
	i9On, _ := parseDate(req.I9VerifiedOn)
	return PersonalFields{
		AddressLine1:   req.AddressLine1,
		AddressLine2:   req.AddressLine2,
		City:           req.City,
		Region:         req.Region,
		PostalCode:     req.PostalCode,
		Country:        req.Country,
		Gender:         req.Gender,
		Ethnicity:      req.Ethnicity,
		MaritalStatus:  req.MaritalStatus,
		WorkAuthType:   req.WorkAuthType,
		WorkAuthExpiry: authExpiry,
		I9Verified:     req.I9Verified,
		I9VerifiedOn:   i9On,
	}
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
		PersonalFields: req.personal(),
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
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		PreferredName:  req.PreferredName,
		WorkEmail:      req.WorkEmail,
		PersonalEmail:  req.PersonalEmail,
		Phone:          req.Phone,
		DateOfBirth:    dob,
		HireDate:       hire,
		Status:         req.Status,
		PersonalFields: req.personal(),
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

func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	prof, err := h.svc.Profile(r.Context(), p.OrgID, id)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "worker not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not fetch profile")
		return
	}
	httpx.JSON(w, http.StatusOK, prof)
}

func (h *Handler) events(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	events, err := h.svc.Events(r.Context(), p.OrgID, id)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "worker not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not fetch events")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": events})
}

type terminateRequest struct {
	EffectiveDate *string `json:"effective_date"`
	Reason        string  `json:"reason"`
}

func (h *Handler) terminate(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	var req terminateRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	eff, effErr := parseDate(req.EffectiveDate)
	if effErr != nil {
		httpx.ValidationError(w, map[string]string{"effective_date": "must be YYYY-MM-DD"})
		return
	}
	var effTime time.Time
	if eff != nil {
		effTime = *eff
	}
	updated, err := h.svc.Terminate(r.Context(), p.OrgID, p.UserID, id, effTime, req.Reason)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "worker not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not terminate worker")
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
