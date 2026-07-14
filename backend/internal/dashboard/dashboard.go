// Package dashboard provides org-level summary metrics for the home screen.
package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// Summary is the home-screen rollup for an organization.
type Summary struct {
	Headcount     int          `json:"headcount"`
	OpenPositions int          `json:"open_positions"`
	Departments   int          `json:"departments"`
	OnLeave       int          `json:"on_leave"`
	OutToday      []OutToday   `json:"out_today"`
	RecentHires   []RecentHire `json:"recent_hires"`
}

// OutToday is a worker on approved leave that covers today.
type OutToday struct {
	WorkerID  uuid.UUID `json:"worker_id"`
	Name      string    `json:"name"`
	LeaveType string    `json:"leave_type"`
	EndDate   time.Time `json:"end_date"`
}

// RecentHire is a recently added worker.
type RecentHire struct {
	WorkerID uuid.UUID  `json:"worker_id"`
	Name     string     `json:"name"`
	HireDate *time.Time `json:"hire_date,omitempty"`
	Title    *string    `json:"title,omitempty"`
}

// Store runs the aggregate queries.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds a dashboard Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Summary computes the org rollup in a handful of scoped queries.
func (s *Store) Summary(ctx context.Context, orgID uuid.UUID) (Summary, error) {
	var sum Summary
	if err := s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE status = 'active'),
			count(*) FILTER (WHERE status = 'on_leave')
		FROM workers WHERE org_id = $1`, orgID).Scan(&sum.Headcount, &sum.OnLeave); err != nil {
		return Summary{}, err
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM positions WHERE org_id=$1 AND status='open'`, orgID).Scan(&sum.OpenPositions); err != nil {
		return Summary{}, err
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM departments WHERE org_id=$1`, orgID).Scan(&sum.Departments); err != nil {
		return Summary{}, err
	}

	// Who's out today: approved time off covering the current date.
	outRows, err := s.pool.Query(ctx, `
		SELECT w.id, w.first_name || ' ' || w.last_name, lt.name, t.end_date
		FROM time_off_requests t
		JOIN workers w ON w.id = t.worker_id
		JOIN leave_types lt ON lt.id = t.leave_type_id
		WHERE t.org_id = $1 AND t.status = 'approved'
		  AND CURRENT_DATE BETWEEN t.start_date AND t.end_date
		ORDER BY w.last_name`, orgID)
	if err != nil {
		return Summary{}, err
	}
	defer outRows.Close()
	sum.OutToday = []OutToday{}
	for outRows.Next() {
		var o OutToday
		if err := outRows.Scan(&o.WorkerID, &o.Name, &o.LeaveType, &o.EndDate); err != nil {
			return Summary{}, err
		}
		sum.OutToday = append(sum.OutToday, o)
	}
	if err := outRows.Err(); err != nil {
		return Summary{}, err
	}

	// Recent hires: last five by hire date (then created_at), with title.
	hireRows, err := s.pool.Query(ctx, `
		SELECT w.id, w.first_name || ' ' || w.last_name, w.hire_date, p.title
		FROM workers w
		LEFT JOIN worker_assignments a ON a.worker_id = w.id AND a.end_date IS NULL AND a.is_primary
		LEFT JOIN positions p ON p.id = a.position_id
		WHERE w.org_id = $1 AND w.status <> 'terminated'
		ORDER BY w.hire_date DESC NULLS LAST, w.created_at DESC
		LIMIT 5`, orgID)
	if err != nil {
		return Summary{}, err
	}
	defer hireRows.Close()
	sum.RecentHires = []RecentHire{}
	for hireRows.Next() {
		var h RecentHire
		if err := hireRows.Scan(&h.WorkerID, &h.Name, &h.HireDate, &h.Title); err != nil {
			return Summary{}, err
		}
		sum.RecentHires = append(sum.RecentHires, h)
	}
	return sum, hireRows.Err()
}

// Handler exposes the dashboard endpoint.
type Handler struct{ store *Store }

// NewHandler builds a dashboard Handler.
func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// Routes mounts the dashboard route.
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission("worker:read")).Get("/", h.summary)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	sum, err := h.store.Summary(r.Context(), p.OrgID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not build dashboard")
		return
	}
	httpx.JSON(w, http.StatusOK, sum)
}
