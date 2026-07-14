package orgstructure

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
	r.With(write).Put("/departments/{id}", h.updateDepartment)
	r.With(write).Delete("/departments/{id}", h.deleteDepartment)
	r.With(read).Get("/locations", h.listLocations)
	r.With(write).Post("/locations", h.createLocation)
	r.With(write).Put("/locations/{id}", h.updateLocation)
	r.With(write).Delete("/locations/{id}", h.deleteLocation)
	r.With(read).Get("/legal-entities", h.listLegalEntities)
	r.With(write).Post("/legal-entities", h.createLegalEntity)
	r.With(write).Put("/legal-entities/{id}", h.updateLegalEntity)
	r.With(write).Delete("/legal-entities/{id}", h.deleteLegalEntity)
	r.With(read).Get("/job-profiles", h.listJobProfiles)
	r.With(write).Post("/job-profiles", h.createJobProfile)
	r.With(write).Put("/job-profiles/{id}", h.updateJobProfile)
	r.With(write).Delete("/job-profiles/{id}", h.deleteJobProfile)
	r.With(read).Get("/positions", h.listPositions)
	r.With(write).Post("/positions", h.createPosition)
	r.With(write).Put("/positions/{id}", h.updatePosition)
	r.With(write).Delete("/positions/{id}", h.deletePosition)
	r.With(write).Post("/assignments", h.createAssignment)
	r.With(read).Get("/assignments", h.listAssignments)      // ?worker_id=
	r.With(read).Get("/assignments/as-of", h.assignmentAsOf) // ?worker_id=&as_of=
	r.With(read).Get("/org-chart", h.orgChart)
}

type legalEntityRequest struct {
	Name    string `json:"name"`
	Country string `json:"country"`
	TaxID   string `json:"tax_id"`
}

func (h *Handler) createLegalEntity(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req legalEntityRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	e, err := h.svc.CreateLegalEntity(r.Context(), p.OrgID, p.UserID, LegalEntity{
		Name: req.Name, Country: req.Country, TaxID: req.TaxID,
	})
	if err != nil {
		writeConflictOrInternal(w, err, "could not create legal entity")
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

func (h *Handler) listLegalEntities(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	e, err := h.svc.ListLegalEntities(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list legal entities")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": e})
}

type jobProfileRequest struct {
	Title      string `json:"title"`
	JobFamily  string `json:"job_family"`
	Level      string `json:"level"`
	FLSAStatus string `json:"flsa_status"`
}

func (h *Handler) createJobProfile(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req jobProfileRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		httpx.ValidationError(w, map[string]string{"title": "title is required"})
		return
	}
	if req.FLSAStatus != "" && req.FLSAStatus != "exempt" && req.FLSAStatus != "non_exempt" {
		httpx.ValidationError(w, map[string]string{"flsa_status": "must be exempt or non_exempt"})
		return
	}
	j, err := h.svc.CreateJobProfile(r.Context(), p.OrgID, p.UserID, JobProfile{
		Title: req.Title, JobFamily: req.JobFamily, Level: req.Level, FLSAStatus: req.FLSAStatus,
	})
	if err != nil {
		writeConflictOrInternal(w, err, "could not create job profile")
		return
	}
	httpx.JSON(w, http.StatusCreated, j)
}

func (h *Handler) listJobProfiles(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	j, err := h.svc.ListJobProfiles(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list job profiles")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": j})
}

func (h *Handler) listAssignments(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	workerID, err := uuid.Parse(r.URL.Query().Get("worker_id"))
	if err != nil {
		httpx.ValidationError(w, map[string]string{"worker_id": "valid worker_id is required"})
		return
	}
	a, err := h.svc.ListAssignments(r.Context(), p.OrgID, workerID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list assignments")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": a})
}

func (h *Handler) assignmentAsOf(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	q := r.URL.Query()
	workerID, err := uuid.Parse(q.Get("worker_id"))
	if err != nil {
		httpx.ValidationError(w, map[string]string{"worker_id": "valid worker_id is required"})
		return
	}
	asOf, err := time.Parse("2006-01-02", q.Get("as_of"))
	if err != nil {
		httpx.ValidationError(w, map[string]string{"as_of": "as_of must be YYYY-MM-DD"})
		return
	}
	a, err := h.svc.AssignmentAsOf(r.Context(), p.OrgID, workerID, asOf)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "no assignment in effect on that date")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not resolve assignment")
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

// parseID parses the {id} URL param, writing a 400 and returning ok=false on
// failure.
func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// writeMutationError maps common update/delete errors to responses: not-found,
// unique-violation (409), and foreign-key-in-use (409).
func writeMutationError(w http.ResponseWriter, err error, msg string) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, ErrSelfParent):
		httpx.ValidationError(w, map[string]string{"parent_id": ErrSelfParent.Error()})
	case err != nil && strings.Contains(err.Error(), "SQLSTATE 23505"):
		httpx.Error(w, http.StatusConflict, "duplicate", "a record with that name already exists")
	case err != nil && strings.Contains(err.Error(), "SQLSTATE 23503"):
		httpx.Error(w, http.StatusConflict, "in_use", "this record is still referenced and cannot be deleted")
	default:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", msg)
	}
}

func (h *Handler) updateDepartment(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
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
	d, err := h.svc.UpdateDepartment(r.Context(), p.OrgID, p.UserID, id, req.Name, req.Code, parentID, req.CostCenter)
	if err != nil {
		writeMutationError(w, err, "could not update department")
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *Handler) deleteDepartment(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteDepartment(r.Context(), p.OrgID, p.UserID, id); err != nil {
		writeMutationError(w, err, "could not delete department")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) updateLocation(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req locationRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	l, err := h.svc.UpdateLocation(r.Context(), p.OrgID, p.UserID, Location{
		ID: id, Name: req.Name, Address: req.Address, City: req.City,
		Region: req.Region, Country: req.Country, Timezone: req.Timezone,
	})
	if err != nil {
		writeMutationError(w, err, "could not update location")
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

func (h *Handler) deleteLocation(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteLocation(r.Context(), p.OrgID, p.UserID, id); err != nil {
		writeMutationError(w, err, "could not delete location")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) updateLegalEntity(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req legalEntityRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	e, err := h.svc.UpdateLegalEntity(r.Context(), p.OrgID, p.UserID, LegalEntity{
		ID: id, Name: req.Name, Country: req.Country, TaxID: req.TaxID,
	})
	if err != nil {
		writeMutationError(w, err, "could not update legal entity")
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *Handler) deleteLegalEntity(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteLegalEntity(r.Context(), p.OrgID, p.UserID, id); err != nil {
		writeMutationError(w, err, "could not delete legal entity")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) updateJobProfile(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req jobProfileRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		httpx.ValidationError(w, map[string]string{"title": "title is required"})
		return
	}
	if req.FLSAStatus != "" && req.FLSAStatus != "exempt" && req.FLSAStatus != "non_exempt" {
		httpx.ValidationError(w, map[string]string{"flsa_status": "must be exempt or non_exempt"})
		return
	}
	j, err := h.svc.UpdateJobProfile(r.Context(), p.OrgID, p.UserID, JobProfile{
		ID: id, Title: req.Title, JobFamily: req.JobFamily, Level: req.Level, FLSAStatus: req.FLSAStatus,
	})
	if err != nil {
		writeMutationError(w, err, "could not update job profile")
		return
	}
	httpx.JSON(w, http.StatusOK, j)
}

func (h *Handler) deleteJobProfile(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteJobProfile(r.Context(), p.OrgID, p.UserID, id); err != nil {
		writeMutationError(w, err, "could not delete job profile")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) updatePosition(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
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
	jobID, ok := optionalUUID(w, req.JobProfileID)
	if !ok {
		return
	}
	entityID, ok := optionalUUID(w, req.LegalEntityID)
	if !ok {
		return
	}
	pos := Position{ID: id, Title: req.Title, DepartmentID: deptID, LocationID: locID,
		JobProfileID: jobID, LegalEntityID: entityID, Status: req.Status}
	if req.FTE != nil {
		pos.FTE = *req.FTE
	}
	updated, err := h.svc.UpdatePosition(r.Context(), p.OrgID, p.UserID, pos)
	if err != nil {
		writeMutationError(w, err, "could not update position")
		return
	}
	httpx.JSON(w, http.StatusOK, updated)
}

func (h *Handler) deletePosition(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeletePosition(r.Context(), p.OrgID, p.UserID, id); err != nil {
		writeMutationError(w, err, "could not delete position")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
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
	Title         string   `json:"title"`
	DepartmentID  *string  `json:"department_id"`
	LocationID    *string  `json:"location_id"`
	JobProfileID  *string  `json:"job_profile_id"`
	LegalEntityID *string  `json:"legal_entity_id"`
	Status        string   `json:"status"`
	FTE           *float64 `json:"fte"`
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
	jobID, ok := optionalUUID(w, req.JobProfileID)
	if !ok {
		return
	}
	entityID, ok := optionalUUID(w, req.LegalEntityID)
	if !ok {
		return
	}
	pos := Position{Title: req.Title, DepartmentID: deptID, LocationID: locID,
		JobProfileID: jobID, LegalEntityID: entityID, Status: req.Status}
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
