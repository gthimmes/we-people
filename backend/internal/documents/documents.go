// Package documents stores and serves worker/organization files with
// org-scoped access control.
package documents

import (
	"context"
	"errors"
	"io"
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

// maxUploadBytes caps a single upload. Blobs live in Postgres for now; object
// storage (S3/GCS) is the production path for large files.
const maxUploadBytes = 10 << 20 // 10 MiB

// ErrNotFound is returned when a document does not exist in the org.
var ErrNotFound = errors.New("document not found")

// Document is file metadata (without the bytes).
type Document struct {
	ID          uuid.UUID  `json:"id"`
	WorkerID    *uuid.UUID `json:"worker_id,omitempty"`
	Name        string     `json:"name"`
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
	UploadedBy  *uuid.UUID `json:"uploaded_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Content is a document's bytes plus the metadata needed to serve it.
type Content struct {
	Name        string
	ContentType string
	Bytes       []byte
}

// --- store ---

// Store provides org-scoped document data access.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds a document Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// List returns document metadata for an org, optionally filtered by worker.
func (s *Store) List(ctx context.Context, orgID uuid.UUID, workerID *uuid.UUID) ([]Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, worker_id, name, content_type, size_bytes, uploaded_by, created_at
		FROM documents
		WHERE org_id = $1 AND ($2::uuid IS NULL OR worker_id = $2)
		ORDER BY created_at DESC`, orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.WorkerID, &d.Name, &d.ContentType, &d.SizeBytes, &d.UploadedBy, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Create inserts a document and its bytes.
func (s *Store) Create(ctx context.Context, orgID uuid.UUID, workerID *uuid.UUID, name, contentType string, content []byte, uploadedBy *uuid.UUID) (Document, error) {
	var d Document
	err := s.pool.QueryRow(ctx, `
		INSERT INTO documents (org_id, worker_id, name, content_type, size_bytes, content, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, worker_id, name, content_type, size_bytes, uploaded_by, created_at`,
		orgID, workerID, name, contentType, int64(len(content)), content, uploadedBy).
		Scan(&d.ID, &d.WorkerID, &d.Name, &d.ContentType, &d.SizeBytes, &d.UploadedBy, &d.CreatedAt)
	return d, err
}

// GetContent returns a document's bytes for download, scoped to the org.
func (s *Store) GetContent(ctx context.Context, orgID, id uuid.UUID) (Content, error) {
	var c Content
	err := s.pool.QueryRow(ctx, `
		SELECT name, content_type, content FROM documents WHERE org_id=$1 AND id=$2`,
		orgID, id).Scan(&c.Name, &c.ContentType, &c.Bytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return Content{}, ErrNotFound
	}
	return c, err
}

// Delete removes a document scoped to the org.
func (s *Store) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM documents WHERE org_id=$1 AND id=$2`, orgID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- service ---

// Service implements document business logic.
type Service struct {
	store *Store
	audit *audit.Logger
}

// NewService builds the document service.
func NewService(store *Store, auditLog *audit.Logger) *Service {
	return &Service{store: store, audit: auditLog}
}

// List returns document metadata (non-nil slice).
func (s *Service) List(ctx context.Context, orgID uuid.UUID, workerID *uuid.UUID) ([]Document, error) {
	docs, err := s.store.List(ctx, orgID, workerID)
	if err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []Document{}
	}
	return docs, nil
}

// Upload stores a new document.
func (s *Service) Upload(ctx context.Context, orgID, actor uuid.UUID, workerID *uuid.UUID, name, contentType string, content []byte) (Document, error) {
	d, err := s.store.Create(ctx, orgID, workerID, name, contentType, content, &actor)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "document.upload", EntityType: "document", EntityID: &d.ID, After: d})
	}
	return d, err
}

// Download returns a document's bytes.
func (s *Service) Download(ctx context.Context, orgID, id uuid.UUID) (Content, error) {
	return s.store.GetContent(ctx, orgID, id)
}

// Delete removes a document.
func (s *Service) Delete(ctx context.Context, orgID, actor, id uuid.UUID) error {
	err := s.store.Delete(ctx, orgID, id)
	if err == nil {
		s.audit.Record(ctx, audit.Entry{OrgID: orgID, ActorUserID: &actor, Action: "document.delete", EntityType: "document", EntityID: &id})
	}
	return err
}

// --- handler ---

// Handler exposes document HTTP endpoints.
type Handler struct{ svc *Service }

// NewHandler builds a document Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes mounts document routes under an authenticated router.
func (h *Handler) Routes(r chi.Router) {
	read := auth.RequirePermission("worker:read")
	write := auth.RequirePermission("worker:write")

	r.With(read).Get("/", h.list)
	r.With(read).Get("/{id}/download", h.download)
	r.With(write).Post("/", h.upload)
	r.With(write).Delete("/{id}", h.delete)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	workerID, ok := optionalUUIDParam(w, r.URL.Query().Get("worker_id"))
	if !ok {
		return
	}
	docs, err := h.svc.List(r.Context(), p.OrgID, workerID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list documents")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": docs})
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	// Cap the request body to guard memory before parsing.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1024)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "too_large", "file exceeds the 10 MiB limit or form is invalid")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.ValidationError(w, map[string]string{"file": "a file is required"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(io.LimitReader(file, maxUploadBytes))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not read file")
		return
	}
	workerID, ok := optionalUUIDParam(w, r.FormValue("worker_id"))
	if !ok {
		return
	}
	name := r.FormValue("name")
	if strings.TrimSpace(name) == "" {
		name = header.Filename
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	doc, err := h.svc.Upload(r.Context(), p.OrgID, p.UserID, workerID, name, contentType, content)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not store document")
		return
	}
	httpx.JSON(w, http.StatusCreated, doc)
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid document id")
		return
	}
	c, err := h.svc.Download(r.Context(), p.OrgID, id)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "document not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not fetch document")
		return
	}
	w.Header().Set("Content-Type", c.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+sanitizeFilename(c.Name)+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(c.Bytes)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid document id")
		return
	}
	err = h.svc.Delete(r.Context(), p.OrgID, p.UserID, id)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "document not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not delete document")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}

// optionalUUIDParam parses an optional UUID from a query/form value.
func optionalUUIDParam(w http.ResponseWriter, s string) (*uuid.UUID, bool) {
	if strings.TrimSpace(s) == "" {
		return nil, true
	}
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		httpx.ValidationError(w, map[string]string{"worker_id": "invalid UUID"})
		return nil, false
	}
	return &id, true
}

// sanitizeFilename strips characters that could break the Content-Disposition
// header or enable header injection.
func sanitizeFilename(name string) string {
	name = strings.NewReplacer("\"", "", "\\", "", "\n", "", "\r", "").Replace(name)
	if name == "" {
		return "download"
	}
	return name
}
