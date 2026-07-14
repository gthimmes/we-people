// Package app wires the modules together into an HTTP handler.
package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gthimmes/we-people/backend/internal/audit"
	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/config"
	"github.com/gthimmes/we-people/backend/internal/database"
	"github.com/gthimmes/we-people/backend/internal/documents"
	"github.com/gthimmes/we-people/backend/internal/httpx"
	"github.com/gthimmes/we-people/backend/internal/iam"
	"github.com/gthimmes/we-people/backend/internal/org"
	"github.com/gthimmes/we-people/backend/internal/orgstructure"
	"github.com/gthimmes/we-people/backend/internal/worker"
)

// App holds the constructed dependency graph.
type App struct {
	Router http.Handler
}

// New builds the full application: stores, services, handlers, and routes.
func New(db *database.DB, cfg config.Config) *App {
	pool := db.Pool

	// Infrastructure
	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	auditLog := audit.NewLogger(pool)

	// Stores
	orgStore := org.NewStore(pool)
	iamStore := iam.NewStore(pool)
	workerStore := worker.NewStore(pool)
	structStore := orgstructure.NewStore(pool)
	docStore := documents.NewStore(pool)

	// Services
	iamSvc := iam.NewService(iamStore, orgStore, tokens)
	workerSvc := worker.NewService(workerStore, auditLog)
	structSvc := orgstructure.NewService(structStore, auditLog)
	docSvc := documents.NewService(docStore, auditLog)

	// Handlers
	iamHandler := iam.NewHandler(iamSvc)
	workerHandler := worker.NewHandler(workerSvc)
	structHandler := orgstructure.NewHandler(structSvc)
	docHandler := documents.NewHandler(docSvc)

	// Auth middleware (loads permissions from the IAM store)
	authMW := auth.NewMiddleware(tokens, iamStore)

	r := chi.NewRouter()
	r.Use(httpx.RequestID)
	r.Use(httpx.Logger)
	r.Use(httpx.Recoverer)
	r.Use(cors)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Public auth endpoints
		r.Route("/auth", iamHandler.Routes)

		// Authenticated endpoints
		r.Group(func(r chi.Router) {
			r.Use(authMW.Authenticate)
			r.Get("/me", iamHandler.Me)
			r.Route("/workers", workerHandler.Routes)
			r.Route("/documents", docHandler.Routes)
			r.Group(structHandler.Routes)
		})
	})

	return &App{Router: r}
}

// cors is a permissive development CORS middleware. Lock this down per-env
// before production.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
