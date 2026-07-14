package approvals

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// Handler exposes approval HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds an approvals Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts approval routes. Any authenticated user may view their own
// inbox and decide on requests assigned to them; the service enforces identity.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.listPending)
	r.Post("/{id}/decide", h.decide)
}

func (h *Handler) listPending(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	reqs, err := h.svc.PendingForUser(r.Context(), p.OrgID, p.UserID, p.Can("org:write"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list approvals")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": reqs})
}

type decideRequest struct {
	Approve bool   `json:"approve"`
	Note    string `json:"note"`
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid approval id")
		return
	}
	var req decideRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	updated, err := h.svc.Decide(r.Context(), p.OrgID, p.UserID, id, req.Approve, req.Note, p.Can("org:write"))
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "approval not found")
	case errors.Is(err, ErrNotAuthorized):
		httpx.Error(w, http.StatusForbidden, "forbidden", "you are not the assigned approver")
	case errors.Is(err, ErrNotPending):
		httpx.Error(w, http.StatusConflict, "not_pending", "this request has already been decided")
	case err != nil:
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not record decision")
	default:
		httpx.JSON(w, http.StatusOK, updated)
	}
}
