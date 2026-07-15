// Package compensation tracks effective-dated pay records per worker.
package compensation

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gthimmes/we-people/backend/internal/audit"
	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// ErrNotFound is returned when no compensation exists.
var ErrNotFound = errors.New("compensation not found")

// Record is a single effective-dated pay record.
type Record struct {
	ID            uuid.UUID `json:"id"`
	WorkerID      uuid.UUID `json:"worker_id"`
	EffectiveDate time.Time `json:"effective_date"`
	PayType       string    `json:"pay_type"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	PayFrequency  string    `json:"pay_frequency"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
}

// --- store ---

// Store provides org-scoped compensation data access.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds a compensation Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const cols = `id, worker_id, effective_date, pay_type, amount, currency, pay_frequency, reason, created_at`

func scan(row pgx.Row) (Record, error) {
	var r Record
	err := row.Scan(&r.ID, &r.WorkerID, &r.EffectiveDate, &r.PayType, &r.Amount, &r.Currency, &r.PayFrequency, &r.Reason, &r.CreatedAt)
	return r, err
}

// History returns a worker's compensation records, newest first.
func (s *Store) History(ctx context.Context, orgID, workerID uuid.UUID) ([]Record, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+cols+` FROM compensation_records WHERE org_id=$1 AND worker_id=$2 ORDER BY effective_date DESC, created_at DESC`,
		orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Create inserts a compensation record.
func (s *Store) Create(ctx context.Context, orgID uuid.UUID, r Record, createdBy uuid.UUID) (Record, error) {
	return scan(s.pool.QueryRow(ctx, `
		INSERT INTO compensation_records (org_id, worker_id, effective_date, pay_type, amount, currency, pay_frequency, reason, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING `+cols,
		orgID, r.WorkerID, r.EffectiveDate, r.PayType, r.Amount, r.Currency, r.PayFrequency, r.Reason, createdBy))
}

// --- service ---

// Notifier delivers a notification to the user linked to a worker (best-effort).
type Notifier interface {
	NotifyWorker(ctx context.Context, orgID, workerID uuid.UUID, typ, title, body, link string)
}

// Service implements compensation business logic.
type Service struct {
	store    *Store
	audit    *audit.Logger
	notifier Notifier
}

// NewService builds the compensation service.
func NewService(store *Store, auditLog *audit.Logger) *Service {
	return &Service{store: store, audit: auditLog}
}

// SetNotifier wires the notification sink (optional).
func (s *Service) SetNotifier(n Notifier) { s.notifier = n }

// Current returns the record in effect today (latest effective_date <= today).
func current(records []Record) *Record {
	today := time.Now()
	for i := range records { // records are newest-effective first
		if !records[i].EffectiveDate.After(today) {
			return &records[i]
		}
	}
	return nil
}

// HistoryResult bundles the current record with the full history.
type HistoryResult struct {
	Current *Record  `json:"current"`
	History []Record `json:"history"`
}

// History returns a worker's compensation with the current record resolved.
func (s *Service) History(ctx context.Context, orgID, workerID uuid.UUID) (HistoryResult, error) {
	records, err := s.store.History(ctx, orgID, workerID)
	if err != nil {
		return HistoryResult{}, err
	}
	if records == nil {
		records = []Record{}
	}
	return HistoryResult{Current: current(records), History: records}, nil
}

// Add records a compensation change and notifies the worker.
func (s *Service) Add(ctx context.Context, orgID, actor uuid.UUID, r Record) (Record, error) {
	created, err := s.store.Create(ctx, orgID, r, actor)
	if err != nil {
		return Record{}, err
	}
	s.audit.Record(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: &actor,
		Action: "compensation.add", EntityType: "compensation_record", EntityID: &created.ID, After: created,
	})
	if s.notifier != nil {
		s.notifier.NotifyWorker(ctx, orgID, r.WorkerID, "compensation.change", "Your compensation was updated", r.Reason, "/directory")
	}
	return created, nil
}

// --- handler ---

// Handler exposes compensation HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds a compensation Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts compensation routes (kept off the /workers subrouter to avoid
// pattern conflicts). Gated by dedicated compensation permissions.
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission("compensation:read")).Get("/compensation", h.history)
	r.With(auth.RequirePermission("compensation:write")).Post("/compensation", h.add)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	workerID, err := uuid.Parse(r.URL.Query().Get("worker_id"))
	if err != nil {
		httpx.ValidationError(w, map[string]string{"worker_id": "valid worker_id is required"})
		return
	}
	res, err := h.svc.History(r.Context(), p.OrgID, workerID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not fetch compensation")
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

type addRequest struct {
	WorkerID      string  `json:"worker_id"`
	EffectiveDate string  `json:"effective_date"`
	PayType       string  `json:"pay_type"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	PayFrequency  string  `json:"pay_frequency"`
	Reason        string  `json:"reason"`
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	var req addRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	fields := map[string]string{}
	workerID, err := uuid.Parse(req.WorkerID)
	if err != nil {
		fields["worker_id"] = "valid worker_id is required"
	}
	eff, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		fields["effective_date"] = "effective_date must be YYYY-MM-DD"
	}
	if req.Amount <= 0 {
		fields["amount"] = "amount must be greater than 0"
	}
	if len(fields) > 0 {
		httpx.ValidationError(w, fields)
		return
	}

	rec := Record{
		WorkerID: workerID, EffectiveDate: eff, Amount: req.Amount,
		PayType: def(req.PayType, "salary"), Currency: def(req.Currency, "USD"),
		PayFrequency: def(req.PayFrequency, "annual"), Reason: req.Reason,
	}
	created, err := h.svc.Add(r.Context(), p.OrgID, p.UserID, rec)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not add compensation")
		return
	}
	httpx.JSON(w, http.StatusCreated, created)
}

func def(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
