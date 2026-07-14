package orgstructure

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// Handler exposes orgstructure HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds an orgstructure Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts orgstructure routes under an authenticated router.
func (h *Handler) Routes(r chi.Router) {
	read := auth.RequirePermission("orgstructure:read")
	write := auth.RequirePermission("orgstructure:write")

	r.With(read).Get("/departments", h.listDepartments)
	r.With(write).Post("/departments", h.createDepartment)
	r.With(read).Get("/locations", h.listLocations)
	r.With(write).Post("/locations", h.createLocation)
	r.With(read).Get("/positions", h.listPositions)
	r.With(write).Post("/positions", h.createPosition)
	r.With(write).Post("/assignments", h.createAssignment)
	r.With(read).Get("/org-chart", h.orgChart)
}

type departmentRequest struct {
	Name       string  `json:"name"`
	Code       string  `json:"code"`
	ParentID   *string `json:"parent_id"`
	CostCenter string  `json:"cost_center"`
}

func (h *Handler) createDepartment(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req departmentRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	parentID, ok := optionalUUID(w, req.ParentID)
	if !ok {
		return
	}
	d, err := h.svc.CreateDepartment(r.Context(), p.OrgID, p.UserID, req.Name, req.Code, parentID, req.CostCenter)
	if err != nil {
		writeConflictOrInternal(w, err, "could not create department")
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

func (h *Handler) listDepartments(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	d, err := h.svc.ListDepartments(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list departments")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": d})
}

type locationRequest struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Timezone string `json:"timezone"`
}

func (h *Handler) createLocation(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req locationRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	l, err := h.svc.CreateLocation(r.Context(), p.OrgID, p.UserID, Location{
		Name: req.Name, Address: req.Address, City: req.City,
		Region: req.Region, Country: req.Country, Timezone: req.Timezone,
	})
	if err != nil {
		writeConflictOrInternal(w, err, "could not create location")
		return
	}
	httpx.JSON(w, http.StatusCreated, l)
}

func (h *Handler) listLocations(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	l, err := h.svc.ListLocations(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list locations")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": l})
}

type positionRequest struct {
	Title        string   `json:"title"`
	DepartmentID *string  `json:"department_id"`
	LocationID   *string  `json:"location_id"`
	Status       string   `json:"status"`
	FTE          *float64 `json:"fte"`
}

func (h *Handler) createPosition(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req positionRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		httpx.ValidationError(w, map[string]string{"title": "title is required"})
		return
	}
	deptID, ok := optionalUUID(w, req.DepartmentID)
	if !ok {
		return
	}
	locID, ok := optionalUUID(w, req.LocationID)
	if !ok {
		return
	}
	pos := Position{Title: req.Title, DepartmentID: deptID, LocationID: locID, Status: req.Status}
	if req.FTE != nil {
		pos.FTE = *req.FTE
	}
	created, err := h.svc.CreatePosition(r.Context(), p.OrgID, p.UserID, pos)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not create position")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func (h *Handler) listPositions(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	pos, err := h.svc.ListPositions(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list positions")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": pos})
}

type assignmentRequest struct {
	WorkerID      string  `json:"worker_id"`
	PositionID    *string `json:"position_id"`
	ManagerID     *string `json:"manager_id"`
	EffectiveDate *string `json:"effective_date"`
	EventType     string  `json:"event_type"`
	Reason        string  `json:"reason"`
}

func (h *Handler) createAssignment(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req assignmentRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	workerID, err := uuid.Parse(req.WorkerID)
	if err != nil {
		httpx.ValidationError(w, map[string]string{"worker_id": "valid worker_id is required"})
		return
	}
	positionID, ok := optionalUUID(w, req.PositionID)
	if !ok {
		return
	}
	managerID, ok := optionalUUID(w, req.ManagerID)
	if !ok {
		return
	}
	var eff time.Time
	if req.EffectiveDate != nil && strings.TrimSpace(*req.EffectiveDate) != "" {
		eff, err = time.Parse("2006-01-02", strings.TrimSpace(*req.EffectiveDate))
		if err != nil {
			httpx.ValidationError(w, map[string]string{"effective_date": "must be YYYY-MM-DD"})
			return
		}
	}
	eventType := req.EventType
	if eventType != "" && !validAssignEvent[eventType] {
		httpx.ValidationError(w, map[string]string{"event_type": "must be one of: transfer, promotion"})
		return
	}
	a, err := h.svc.Assign(r.Context(), p.OrgID, p.UserID, AssignInput{
		WorkerID: workerID, PositionID: positionID, ManagerID: managerID, EffectiveDate: eff,
		EventType: eventType, Reason: req.Reason,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not create assignment")
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

func (h *Handler) orgChart(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	tree, err := h.svc.OrgChart(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not build org chart")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": tree})
}

// validAssignEvent restricts the lifecycle event types an assignment change may
// record. Other employment events (hire, termination) are recorded elsewhere.
var validAssignEvent = map[string]bool{"transfer": true, "promotion": true}

// optionalUUID parses an optional UUID string pointer. Returns (nil, true) when
// absent/empty, and writes a 422 + returns ok=false on a malformed value.
func optionalUUID(w http.ResponseWriter, s *string) (*uuid.UUID, bool) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, true
	}
	id, err := uuid.Parse(strings.TrimSpace(*s))
	if err != nil {
		httpx.ValidationError(w, map[string]string{"id": "invalid UUID: " + *s})
		return nil, false
	}
	return &id, true
}

func writeConflictOrInternal(w http.ResponseWriter, err error, msg string) {
	if strings.Contains(err.Error(), "SQLSTATE 23505") {
		httpx.Error(w, http.StatusConflict, "duplicate", "a record with that name already exists")
		return
	}
	httpx.Error(w, http.StatusInternalServerError, "internal_error", msg)
}
