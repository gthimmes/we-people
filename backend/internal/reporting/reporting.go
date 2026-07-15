// Package reporting provides org-level people analytics: headcount, turnover,
// diversity, compensation, and time-off metrics.
package reporting

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// Bucket is a labeled count for a distribution.
type Bucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// MonthPoint is one month of hires vs terminations.
type MonthPoint struct {
	Month        string `json:"month"`
	Hires        int    `json:"hires"`
	Terminations int    `json:"terminations"`
}

// CompBucket is average compensation for a group.
type CompBucket struct {
	Label string  `json:"label"`
	Avg   float64 `json:"avg"`
	Count int     `json:"count"`
}

// CompReport holds compensation analytics (omitted without permission).
type CompReport struct {
	AvgAnnual float64      `json:"avg_annual"`
	ByDept    []CompBucket `json:"by_department"`
}

// TimeOffReport holds leave analytics.
type TimeOffReport struct {
	BalanceLiabilityHours float64 `json:"balance_liability_hours"`
	PendingRequests       int     `json:"pending_requests"`
	ApprovedUpcomingHours float64 `json:"approved_upcoming_hours"`
}

// Report is the full analytics rollup.
type Report struct {
	Headcount        int           `json:"headcount"`
	HeadcountByDept  []Bucket      `json:"headcount_by_department"`
	HeadcountByLoc   []Bucket      `json:"headcount_by_location"`
	StatusBreakdown  []Bucket      `json:"status_breakdown"`
	GenderBreakdown  []Bucket      `json:"gender_breakdown"`
	Hires12mo        int           `json:"hires_12mo"`
	Terminations12mo int           `json:"terminations_12mo"`
	TurnoverRate     float64       `json:"turnover_rate"` // terminations / active headcount
	MonthlyTrend     []MonthPoint  `json:"monthly_trend"`
	TimeOff          TimeOffReport `json:"time_off"`
	Comp             *CompReport   `json:"compensation,omitempty"`
}

// Store runs the aggregate queries.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds a reporting Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) buckets(ctx context.Context, sql string, orgID uuid.UUID) ([]Bucket, error) {
	rows, err := s.pool.Query(ctx, sql, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Bucket{}
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Label, &b.Count); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Report assembles the analytics rollup. Compensation is included only when
// includeComp is set (the caller holds compensation:read).
func (s *Store) Report(ctx context.Context, orgID uuid.UUID, includeComp bool) (Report, error) {
	var r Report
	var err error

	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM workers WHERE org_id=$1 AND status='active'`, orgID).Scan(&r.Headcount); err != nil {
		return r, err
	}

	if r.HeadcountByDept, err = s.buckets(ctx, `
		SELECT COALESCE(d.name, 'Unassigned'), count(*)
		FROM workers w
		LEFT JOIN worker_assignments a ON a.worker_id=w.id AND a.end_date IS NULL AND a.is_primary
		LEFT JOIN positions p ON p.id=a.position_id
		LEFT JOIN departments d ON d.id=p.department_id
		WHERE w.org_id=$1 AND w.status='active'
		GROUP BY d.name ORDER BY count(*) DESC, d.name`, orgID); err != nil {
		return r, err
	}

	if r.HeadcountByLoc, err = s.buckets(ctx, `
		SELECT COALESCE(l.name, 'Unassigned'), count(*)
		FROM workers w
		LEFT JOIN worker_assignments a ON a.worker_id=w.id AND a.end_date IS NULL AND a.is_primary
		LEFT JOIN positions p ON p.id=a.position_id
		LEFT JOIN locations l ON l.id=p.location_id
		WHERE w.org_id=$1 AND w.status='active'
		GROUP BY l.name ORDER BY count(*) DESC, l.name`, orgID); err != nil {
		return r, err
	}

	if r.StatusBreakdown, err = s.buckets(ctx,
		`SELECT status, count(*) FROM workers WHERE org_id=$1 GROUP BY status ORDER BY count(*) DESC`, orgID); err != nil {
		return r, err
	}

	if r.GenderBreakdown, err = s.buckets(ctx, `
		SELECT CASE WHEN gender = '' THEN 'Not specified' ELSE gender END, count(*)
		FROM workers WHERE org_id=$1 AND status='active'
		GROUP BY 1 ORDER BY count(*) DESC`, orgID); err != nil {
		return r, err
	}

	// Hires / terminations over the trailing 12 months.
	if err = s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE type='hire'),
			count(*) FILTER (WHERE type='termination')
		FROM lifecycle_events
		WHERE org_id=$1 AND effective_date >= CURRENT_DATE - INTERVAL '12 months'`, orgID).
		Scan(&r.Hires12mo, &r.Terminations12mo); err != nil {
		return r, err
	}
	if r.Headcount > 0 {
		r.TurnoverRate = float64(r.Terminations12mo) / float64(r.Headcount)
	}

	trendRows, err := s.pool.Query(ctx, `
		SELECT to_char(effective_date, 'YYYY-MM'),
			count(*) FILTER (WHERE type='hire'),
			count(*) FILTER (WHERE type='termination')
		FROM lifecycle_events
		WHERE org_id=$1 AND effective_date >= CURRENT_DATE - INTERVAL '12 months'
		GROUP BY 1 ORDER BY 1`, orgID)
	if err != nil {
		return r, err
	}
	r.MonthlyTrend = []MonthPoint{}
	for trendRows.Next() {
		var m MonthPoint
		if err := trendRows.Scan(&m.Month, &m.Hires, &m.Terminations); err != nil {
			trendRows.Close()
			return r, err
		}
		r.MonthlyTrend = append(r.MonthlyTrend, m)
	}
	trendRows.Close()
	if err := trendRows.Err(); err != nil {
		return r, err
	}

	// Time off.
	if err = s.pool.QueryRow(ctx,
		`SELECT COALESCE(sum(balance_hours), 0) FROM leave_balances WHERE org_id=$1`, orgID).
		Scan(&r.TimeOff.BalanceLiabilityHours); err != nil {
		return r, err
	}
	if err = s.pool.QueryRow(ctx,
		`SELECT count(*) FROM time_off_requests WHERE org_id=$1 AND status='pending'`, orgID).
		Scan(&r.TimeOff.PendingRequests); err != nil {
		return r, err
	}
	if err = s.pool.QueryRow(ctx,
		`SELECT COALESCE(sum(hours), 0) FROM time_off_requests WHERE org_id=$1 AND status='approved' AND end_date >= CURRENT_DATE`, orgID).
		Scan(&r.TimeOff.ApprovedUpcomingHours); err != nil {
		return r, err
	}

	if includeComp {
		comp, err := s.compReport(ctx, orgID)
		if err != nil {
			return r, err
		}
		r.Comp = comp
	}
	return r, nil
}

func (s *Store) compReport(ctx context.Context, orgID uuid.UUID) (*CompReport, error) {
	cr := &CompReport{ByDept: []CompBucket{}}
	// Overall average of each active worker's current annual-equivalent comp.
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(avg(c.amount), 0)
		FROM workers w
		JOIN LATERAL (
			SELECT amount FROM compensation_records cr
			WHERE cr.worker_id = w.id AND cr.effective_date <= CURRENT_DATE
			ORDER BY effective_date DESC LIMIT 1
		) c ON true
		WHERE w.org_id=$1 AND w.status='active'`, orgID).Scan(&cr.AvgAnnual); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(d.name, 'Unassigned'), avg(c.amount), count(*)
		FROM workers w
		JOIN LATERAL (
			SELECT amount FROM compensation_records cr
			WHERE cr.worker_id = w.id AND cr.effective_date <= CURRENT_DATE
			ORDER BY effective_date DESC LIMIT 1
		) c ON true
		LEFT JOIN worker_assignments a ON a.worker_id=w.id AND a.end_date IS NULL AND a.is_primary
		LEFT JOIN positions p ON p.id=a.position_id
		LEFT JOIN departments d ON d.id=p.department_id
		WHERE w.org_id=$1 AND w.status='active'
		GROUP BY d.name ORDER BY avg(c.amount) DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b CompBucket
		if err := rows.Scan(&b.Label, &b.Avg, &b.Count); err != nil {
			return nil, err
		}
		cr.ByDept = append(cr.ByDept, b)
	}
	return cr, rows.Err()
}

// Handler exposes the reporting endpoint.
type Handler struct{ store *Store }

// NewHandler builds a reporting Handler.
func NewHandler(store *Store) *Handler { return &Handler{store: store} }

// Routes mounts the analytics route.
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequirePermission("worker:read")).Get("/reports", h.report)
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	rep, err := h.store.Report(r.Context(), p.OrgID, p.Can("compensation:read"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not build report")
		return
	}
	httpx.JSON(w, http.StatusOK, rep)
}
