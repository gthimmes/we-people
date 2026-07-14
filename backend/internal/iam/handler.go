package iam

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// Handler exposes IAM HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds an IAM Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts the public auth routes (no auth required).
func (h *Handler) Routes(r chi.Router) {
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Post("/logout", h.logout)
}

type registerRequest struct {
	OrgName  string `json:"org_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if fields := validateRegister(req); len(fields) > 0 {
		httpx.ValidationError(w, fields)
		return
	}
	res, err := h.svc.Register(r.Context(), req.OrgName, strings.ToLower(req.Email), req.Password)
	switch {
	case errors.Is(err, ErrSlugTaken):
		httpx.Error(w, http.StatusConflict, "slug_taken", "an organization with a similar name already exists")
		return
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not register organization")
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

type loginRequest struct {
	Slug     string `json:"slug"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	pair, user, err := h.svc.Login(r.Context(), req.Slug, strings.ToLower(req.Email), req.Password)
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
		return
	case errors.Is(err, ErrUserDisabled):
		httpx.Error(w, http.StatusForbidden, "user_disabled", "this account is disabled")
		return
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not log in")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"token": pair, "user": user})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if errors.Is(err, ErrTokenInvalid) {
		httpx.Error(w, http.StatusUnauthorized, "invalid_token", "refresh token is invalid or expired")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not refresh token")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"token": pair})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not log out")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

// AdminRoutes mounts authenticated user/role management under the given router.
func (h *Handler) AdminRoutes(r chi.Router) {
	read := auth.RequirePermission("user:read")
	write := auth.RequirePermission("user:write")
	r.With(read).Get("/users", h.listUsers)
	r.With(write).Post("/users", h.inviteUser)
	r.With(write).Put("/users/{id}/roles", h.setUserRoles)
	r.With(write).Post("/users/{id}/status", h.setUserStatus)
	r.With(read).Get("/roles", h.listRoles)
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	users, err := h.svc.ListUsers(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list users")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": users})
}

func (h *Handler) listRoles(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	roles, err := h.svc.ListRoles(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list roles")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": roles})
}

type inviteUserRequest struct {
	Email    string   `json:"email"`
	Password string   `json:"password"`
	WorkerID *string  `json:"worker_id"`
	RoleIDs  []string `json:"role_ids"`
}

func (h *Handler) inviteUser(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req inviteUserRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	fields := map[string]string{}
	if !strings.Contains(req.Email, "@") {
		fields["email"] = "a valid email is required"
	}
	if len(req.Password) < 8 {
		fields["password"] = "password must be at least 8 characters"
	}
	if len(fields) > 0 {
		httpx.ValidationError(w, fields)
		return
	}
	workerID, err := optionalUUID(req.WorkerID)
	if err != nil {
		httpx.ValidationError(w, map[string]string{"worker_id": "invalid worker id"})
		return
	}
	roleIDs, err := parseUUIDs(req.RoleIDs)
	if err != nil {
		httpx.ValidationError(w, map[string]string{"role_ids": "invalid role id"})
		return
	}
	user, err := h.svc.InviteUser(r.Context(), p.OrgID, strings.ToLower(req.Email), req.Password, workerID, roleIDs)
	if errors.Is(err, ErrEmailTaken) {
		httpx.Error(w, http.StatusConflict, "email_taken", "a user with that email already exists")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not create user")
		return
	}
	httpx.JSON(w, http.StatusCreated, user)
}

type setRolesRequest struct {
	RoleIDs []string `json:"role_ids"`
}

func (h *Handler) setUserRoles(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	var req setRolesRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	roleIDs, err := parseUUIDs(req.RoleIDs)
	if err != nil {
		httpx.ValidationError(w, map[string]string{"role_ids": "invalid role id"})
		return
	}
	if err := h.svc.SetUserRoles(r.Context(), p.OrgID, id, roleIDs); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not update roles")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

type setStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handler) setUserStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid user id")
		return
	}
	var req setStatusRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if req.Status != "active" && req.Status != "disabled" {
		httpx.ValidationError(w, map[string]string{"status": "must be active or disabled"})
		return
	}
	if id == p.UserID && req.Status == "disabled" {
		httpx.Error(w, http.StatusBadRequest, "cannot_disable_self", "you cannot disable your own account")
		return
	}
	if err := h.svc.SetUserStatus(r.Context(), p.OrgID, id, req.Status); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not update status")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func optionalUUID(s *string) (*uuid.UUID, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*s))
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseUUIDs(ss []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(ss))
	for _, s := range ss {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// Me returns the current authenticated principal's identity + permissions.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	if p == nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	perms := make([]string, 0, len(p.Permissions))
	for k := range p.Permissions {
		perms = append(perms, k)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"user_id":     p.UserID,
		"org_id":      p.OrgID,
		"email":       p.Email,
		"permissions": perms,
	})
}

func validateRegister(req registerRequest) map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(req.OrgName) == "" {
		fields["org_name"] = "organization name is required"
	}
	if !strings.Contains(req.Email, "@") {
		fields["email"] = "a valid email is required"
	}
	if len(req.Password) < 8 {
		fields["password"] = "password must be at least 8 characters"
	}
	return fields
}
