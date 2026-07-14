package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gthimmes/we-people/backend/db"
	"github.com/gthimmes/we-people/backend/internal/app"
	"github.com/gthimmes/we-people/backend/internal/config"
	"github.com/gthimmes/we-people/backend/internal/database"
)

// testServer connects to the configured Postgres, ensures migrations are
// applied, and returns an httptest server backed by the real app router. It
// skips the test if the database is unreachable.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	ctx := context.Background()
	conn, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("skipping integration test; database unavailable: %v", err)
	}
	migrations, err := database.LoadMigrations(db.MigrationsFS())
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if err := database.MigrateUp(ctx, conn.Pool, migrations); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	srv := httptest.NewServer(app.New(conn, cfg).Router)
	t.Cleanup(func() { srv.Close(); conn.Close() })
	return srv
}

func TestRegisterLoginAndWorkerFlow(t *testing.T) {
	srv := testServer(t)

	// Unique org name per run so repeated runs don't collide.
	orgName := fmt.Sprintf("Test Co %d", time.Now().UnixNano())

	// Register.
	var reg struct {
		Organization struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"organization"`
		Token struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"token"`
	}
	status := postJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]any{
		"org_name": orgName,
		"email":    "admin@test.co",
		"password": "password123",
	}, &reg)
	if status != http.StatusCreated {
		t.Fatalf("register status = %d", status)
	}
	if reg.Token.AccessToken == "" {
		t.Fatal("register returned no access token")
	}
	access := reg.Token.AccessToken

	// /me should surface the admin's permissions.
	var me struct {
		Permissions []string `json:"permissions"`
	}
	if status := getJSON(t, srv.URL+"/api/v1/me", access, &me); status != http.StatusOK {
		t.Fatalf("/me status = %d", status)
	}
	if !contains(me.Permissions, "worker:write") {
		t.Errorf("admin missing worker:write; got %v", me.Permissions)
	}

	// Unauthenticated worker list is rejected.
	if status := getJSON(t, srv.URL+"/api/v1/workers", "", nil); status != http.StatusUnauthorized {
		t.Errorf("unauthenticated list status = %d, want 401", status)
	}

	// Create a worker.
	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	status = postJSON(t, srv.URL+"/api/v1/workers", access, map[string]any{
		"employee_number": "T-001",
		"first_name":      "Test",
		"last_name":       "Person",
		"work_email":      "tp@test.co",
		"hire_date":       "2024-01-01",
	}, &created)
	if status != http.StatusCreated {
		t.Fatalf("create worker status = %d", status)
	}
	if created.Status != "active" {
		t.Errorf("worker status = %q, want active", created.Status)
	}

	// Duplicate employee number conflicts.
	if status := postJSON(t, srv.URL+"/api/v1/workers", access, map[string]any{
		"employee_number": "T-001", "first_name": "Dup", "last_name": "Licate",
	}, nil); status != http.StatusConflict {
		t.Errorf("duplicate create status = %d, want 409", status)
	}

	// List reflects the new worker.
	var list struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if status := getJSON(t, srv.URL+"/api/v1/workers", access, &list); status != http.StatusOK {
		t.Fatalf("list status = %d", status)
	}
	if list.Meta.Total < 1 {
		t.Errorf("expected at least 1 worker, got %d", list.Meta.Total)
	}

	// Refresh token rotates and yields a new access token.
	var refreshed struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	if status := postJSON(t, srv.URL+"/api/v1/auth/refresh", "", map[string]any{
		"refresh_token": reg.Token.RefreshToken,
	}, &refreshed); status != http.StatusOK {
		t.Fatalf("refresh status = %d", status)
	}
	if refreshed.Token.AccessToken == "" {
		t.Error("refresh returned no access token")
	}
	// Reusing a rotated (now revoked) refresh token must fail.
	if status := postJSON(t, srv.URL+"/api/v1/auth/refresh", "", map[string]any{
		"refresh_token": reg.Token.RefreshToken,
	}, nil); status != http.StatusUnauthorized {
		t.Errorf("reused refresh token status = %d, want 401", status)
	}
}

// --- helpers ---

func postJSON(t *testing.T, url, token string, body any, out any) int {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return do(t, req, out)
}

func getJSON(t *testing.T, url, token string, out any) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return do(t, req, out)
}

func do(t *testing.T, req *http.Request, out any) int {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if out != nil {
		data, _ := io.ReadAll(resp.Body)
		if len(data) > 0 && resp.StatusCode < 300 {
			if err := json.Unmarshal(data, out); err != nil {
				t.Fatalf("decode response (%s): %v", string(data), err)
			}
		}
	}
	return resp.StatusCode
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func TestHealth(t *testing.T) {
	srv := testServer(t)
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d", resp.StatusCode)
	}
}
