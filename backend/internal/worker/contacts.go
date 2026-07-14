package worker

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// EmergencyContact is a person to reach in an emergency for a worker.
type EmergencyContact struct {
	ID           uuid.UUID `json:"id"`
	WorkerID     uuid.UUID `json:"worker_id"`
	Name         string    `json:"name"`
	Relationship string    `json:"relationship"`
	Phone        string    `json:"phone"`
	Email        string    `json:"email"`
	IsPrimary    bool      `json:"is_primary"`
	CreatedAt    time.Time `json:"created_at"`
}

// --- store ---

// ListContacts returns a worker's emergency contacts.
func (s *Store) ListContacts(ctx context.Context, orgID, workerID uuid.UUID) ([]EmergencyContact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, worker_id, name, relationship, phone, email, is_primary, created_at
		FROM emergency_contacts
		WHERE org_id = $1 AND worker_id = $2
		ORDER BY is_primary DESC, name`, orgID, workerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EmergencyContact
	for rows.Next() {
		var c EmergencyContact
		if err := rows.Scan(&c.ID, &c.WorkerID, &c.Name, &c.Relationship, &c.Phone, &c.Email, &c.IsPrimary, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateContact inserts an emergency contact.
func (s *Store) CreateContact(ctx context.Context, orgID uuid.UUID, c EmergencyContact) (EmergencyContact, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO emergency_contacts (org_id, worker_id, name, relationship, phone, email, is_primary)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, worker_id, name, relationship, phone, email, is_primary, created_at`,
		orgID, c.WorkerID, c.Name, c.Relationship, c.Phone, c.Email, c.IsPrimary).
		Scan(&c.ID, &c.WorkerID, &c.Name, &c.Relationship, &c.Phone, &c.Email, &c.IsPrimary, &c.CreatedAt)
	return c, err
}

// DeleteContact removes an emergency contact scoped to org + worker.
func (s *Store) DeleteContact(ctx context.Context, orgID, workerID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM emergency_contacts WHERE org_id=$1 AND worker_id=$2 AND id=$3`,
		orgID, workerID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- service ---

// ListContacts returns a worker's emergency contacts (non-nil slice).
func (s *Service) ListContacts(ctx context.Context, orgID, workerID uuid.UUID) ([]EmergencyContact, error) {
	if _, err := s.store.GetByID(ctx, orgID, workerID); err != nil {
		return nil, err
	}
	cs, err := s.store.ListContacts(ctx, orgID, workerID)
	if err != nil {
		return nil, err
	}
	if cs == nil {
		cs = []EmergencyContact{}
	}
	return cs, nil
}

// AddContact adds an emergency contact to a worker.
func (s *Service) AddContact(ctx context.Context, orgID, workerID uuid.UUID, c EmergencyContact) (EmergencyContact, error) {
	if _, err := s.store.GetByID(ctx, orgID, workerID); err != nil {
		return EmergencyContact{}, err
	}
	c.WorkerID = workerID
	return s.store.CreateContact(ctx, orgID, c)
}

// RemoveContact deletes an emergency contact.
func (s *Service) RemoveContact(ctx context.Context, orgID, workerID, id uuid.UUID) error {
	return s.store.DeleteContact(ctx, orgID, workerID, id)
}

// --- handler ---

func (h *Handler) listContacts(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	workerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	cs, err := h.svc.ListContacts(r.Context(), p.OrgID, workerID)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "worker not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not list contacts")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": cs})
}

type contactRequest struct {
	Name         string `json:"name"`
	Relationship string `json:"relationship"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	IsPrimary    bool   `json:"is_primary"`
}

func (h *Handler) addContact(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	workerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	var req contactRequest
	if !httpx.Decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.ValidationError(w, map[string]string{"name": "name is required"})
		return
	}
	c, err := h.svc.AddContact(r.Context(), p.OrgID, workerID, EmergencyContact{
		Name: req.Name, Relationship: req.Relationship, Phone: req.Phone,
		Email: req.Email, IsPrimary: req.IsPrimary,
	})
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "worker not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not add contact")
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (h *Handler) deleteContact(w http.ResponseWriter, r *http.Request) {
	p := auth.PrincipalFrom(r.Context())
	workerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid worker id")
		return
	}
	contactID, err := uuid.Parse(chi.URLParam(r, "contactId"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid contact id")
		return
	}
	err = h.svc.RemoveContact(r.Context(), p.OrgID, workerID, contactID)
	if errors.Is(err, ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, "not_found", "contact not found")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not delete contact")
		return
	}
	httpx.JSON(w, http.StatusNoContent, nil)
}
