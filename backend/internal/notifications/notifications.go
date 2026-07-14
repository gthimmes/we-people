// Package notifications delivers in-app notifications. Email/push delivery can
// be layered on later behind the same service API.
package notifications

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// ErrNotFound is returned when a notification does not exist for the user.
var ErrNotFound = errors.New("notification not found")

// Notification is a single in-app message for a user.
type Notification struct {
	ID        uuid.UUID  `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Link      string     `json:"link"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// --- store ---

// Store provides data access for notifications.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds a notifications Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) create(ctx context.Context, orgID, userID uuid.UUID, typ, title, body, link string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notifications (org_id, recipient_user_id, type, title, body, link)
		VALUES ($1,$2,$3,$4,$5,$6)`, orgID, userID, typ, title, body, link)
	return err
}

// userForWorker returns the user linked to a worker, if any.
func (s *Store) userForWorker(ctx context.Context, orgID, workerID uuid.UUID) (*uuid.UUID, error) {
	var uid *uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE org_id=$1 AND worker_id=$2 LIMIT 1`, orgID, workerID).Scan(&uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return uid, nil
}

func (s *Store) list(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit int) ([]Notification, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, title, body, link, read_at, created_at
		FROM notifications
		WHERE recipient_user_id=$1 AND ($2 = false OR read_at IS NULL)
		ORDER BY created_at DESC
		LIMIT $3`, userID, unreadOnly, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &n.Link, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) unreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE recipient_user_id=$1 AND read_at IS NULL`, userID).Scan(&n)
	return n, err
}

func (s *Store) markRead(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE notifications SET read_at=now() WHERE recipient_user_id=$1 AND id=$2 AND read_at IS NULL`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) markAllRead(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE notifications SET read_at=now() WHERE recipient_user_id=$1 AND read_at IS NULL`, userID)
	return err
}

// --- service ---

// Service creates and reads notifications. Creation is best-effort: it must
// never fail a caller's primary operation, so errors are logged, not returned.
type Service struct{ store *Store }

// NewService builds the notifications service.
func NewService(store *Store) *Service { return &Service{store: store} }

// NotifyUser sends a notification directly to a user (best-effort).
func (s *Service) NotifyUser(ctx context.Context, orgID, userID uuid.UUID, typ, title, body, link string) {
	if err := s.store.create(ctx, orgID, userID, typ, title, body, link); err != nil {
		slog.Error("notify user", "error", err, "type", typ)
	}
}

// NotifyWorker sends a notification to the user linked to a worker, if any
// (best-effort). Satisfies the Notifier interface used by other modules.
func (s *Service) NotifyWorker(ctx context.Context, orgID, workerID uuid.UUID, typ, title, body, link string) {
	uid, err := s.store.userForWorker(ctx, orgID, workerID)
	if err != nil {
		slog.Error("notify worker lookup", "error", err)
		return
	}
	if uid == nil {
		return // worker has no login; nothing to deliver in-app
	}
	s.NotifyUser(ctx, orgID, *uid, typ, title, body, link)
}

// List returns a user's notifications.
func (s *Service) List(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	ns, err := s.store.list(ctx, userID, unreadOnly, limit)
	if ns == nil {
		ns = []Notification{}
	}
	return ns, err
}

// UnreadCount returns the number of unread notifications for a user.
func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.store.unreadCount(ctx, userID)
}

// MarkRead marks one notification read.
func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	return s.store.markRead(ctx, userID, id)
}

// MarkAllRead marks all of a user's notifications read.
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.store.markAllRead(ctx, userID)
}

// --- handler ---

// Handler exposes notification HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds a notifications Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts notification routes for the authenticated user.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Get("/unread-count", h.unreadCount)
	r.Post("/{id}/read", h.markRead)
	r.Post("/read-all", h.markAllRead)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	unread := r.URL.Query().Get("unread") == "true"
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	ns, err := h.svc.List(r.Context(), p.UserID, unread, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list notifications")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": ns})
}

func (h *Handler) unreadCount(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	n, err := h.svc.UnreadCount(r.Context(), p.UserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not count notifications")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"unread": n})
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid notification id")
		return
	}
	if err := h.svc.MarkRead(r.Context(), p.UserID, id); err != nil && !errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not mark read")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

func (h *Handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	if err := h.svc.MarkAllRead(r.Context(), p.UserID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not mark all read")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
