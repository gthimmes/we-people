// Package org owns organizations — the tenant boundary of the platform.
package org

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when an organization does not exist.
var ErrNotFound = errors.New("organization not found")

// Organization is a tenant.
type Organization struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Slug            string    `json:"slug"`
	FiscalYearStart int       `json:"fiscal_year_start"`
	DefaultCurrency string    `json:"default_currency"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

// Store provides data access for organizations.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds an org Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// CreateTx inserts an organization within an existing transaction.
func (s *Store) CreateTx(ctx context.Context, tx pgx.Tx, name, slug string) (Organization, error) {
	var o Organization
	err := tx.QueryRow(ctx, `
		INSERT INTO organizations (name, slug)
		VALUES ($1, $2)
		RETURNING id, name, slug, fiscal_year_start, default_currency, status, created_at`,
		name, slug).Scan(&o.ID, &o.Name, &o.Slug, &o.FiscalYearStart, &o.DefaultCurrency, &o.Status, &o.CreatedAt)
	return o, err
}

// GetByID returns an organization by id.
func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (Organization, error) {
	var o Organization
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, fiscal_year_start, default_currency, status, created_at
		FROM organizations WHERE id = $1`, id).
		Scan(&o.ID, &o.Name, &o.Slug, &o.FiscalYearStart, &o.DefaultCurrency, &o.Status, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, ErrNotFound
	}
	return o, err
}
