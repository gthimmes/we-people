package iam

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

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
