package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestWorkerLifecycle(t *testing.T) {
	srv := testServer(t)

	// Register a fresh org.
	var reg struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]any{
		"org_name": fmt.Sprintf("Lifecycle Co %d", time.Now().UnixNano()),
		"email":    "hr@lifecycle.co",
		"password": "password123",
	}, &reg); s != http.StatusCreated {
		t.Fatalf("register status = %d", s)
	}
	access := reg.Token.AccessToken

	// Hire a worker.
	var wk struct {
		ID string `json:"id"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/workers", access, map[string]any{
		"employee_number": "L-001", "first_name": "Lee", "last_name": "Cycle",
		"hire_date": "2024-05-01",
	}, &wk); s != http.StatusCreated {
		t.Fatalf("create worker status = %d", s)
	}

	// Profile reflects the worker (no assignment yet, so no title).
	var prof struct {
		Status        string  `json:"status"`
		PositionTitle *string `json:"position_title"`
	}
	if s := getJSON(t, srv.URL+"/api/v1/workers/"+wk.ID+"/profile", access, &prof); s != http.StatusOK {
		t.Fatalf("profile status = %d", s)
	}
	if prof.Status != "active" {
		t.Errorf("profile status = %q, want active", prof.Status)
	}

	// Hire event exists.
	var events struct {
		Data []struct {
			Type string `json:"type"`
		} `json:"data"`
	}
	if s := getJSON(t, srv.URL+"/api/v1/workers/"+wk.ID+"/events", access, &events); s != http.StatusOK {
		t.Fatalf("events status = %d", s)
	}
	if len(events.Data) != 1 || events.Data[0].Type != "hire" {
		t.Errorf("expected one hire event, got %+v", events.Data)
	}

	// Terminate.
	var termed struct {
		Status string `json:"status"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/workers/"+wk.ID+"/terminate", access, map[string]any{
		"effective_date": "2025-01-31", "reason": "test",
	}, &termed); s != http.StatusOK {
		t.Fatalf("terminate status = %d", s)
	}
	if termed.Status != "terminated" {
		t.Errorf("status after terminate = %q, want terminated", termed.Status)
	}

	// Now two events, newest (termination) first.
	if s := getJSON(t, srv.URL+"/api/v1/workers/"+wk.ID+"/events", access, &events); s != http.StatusOK {
		t.Fatalf("events status = %d", s)
	}
	if len(events.Data) != 2 || events.Data[0].Type != "termination" {
		t.Errorf("expected termination then hire, got %+v", events.Data)
	}
}

func TestPersonalDataContactsAndDocuments(t *testing.T) {
	srv := testServer(t)

	var reg struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]any{
		"org_name": fmt.Sprintf("Docs Co %d", time.Now().UnixNano()),
		"email":    "hr@docs.co", "password": "password123",
	}, &reg); s != http.StatusCreated {
		t.Fatalf("register status = %d", s)
	}
	access := reg.Token.AccessToken

	var wk struct {
		ID string `json:"id"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/workers", access, map[string]any{
		"employee_number": "D-001", "first_name": "Dee", "last_name": "Ocean",
	}, &wk); s != http.StatusCreated {
		t.Fatalf("create worker status = %d", s)
	}

	// Update personal fields (address + demographics) and read them back.
	if s := putJSON(t, srv.URL+"/api/v1/workers/"+wk.ID, access, map[string]any{
		"employee_number": "D-001", "first_name": "Dee", "last_name": "Ocean",
		"city": "Denver", "region": "CO", "country": "US", "gender": "female",
	}, nil); s != http.StatusOK {
		t.Fatalf("update status = %d", s)
	}
	var prof struct {
		City   string `json:"city"`
		Gender string `json:"gender"`
	}
	getJSON(t, srv.URL+"/api/v1/workers/"+wk.ID+"/profile", access, &prof)
	if prof.City != "Denver" || prof.Gender != "female" {
		t.Errorf("personal fields not persisted: %+v", prof)
	}

	// Emergency contact.
	if s := postJSON(t, srv.URL+"/api/v1/workers/"+wk.ID+"/emergency-contacts", access, map[string]any{
		"name": "Sky Ocean", "relationship": "Sibling", "phone": "555-1000",
	}, nil); s != http.StatusCreated {
		t.Fatalf("add contact status = %d", s)
	}
	var contacts struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/workers/"+wk.ID+"/emergency-contacts", access, &contacts)
	if len(contacts.Data) != 1 || contacts.Data[0].Name != "Sky Ocean" {
		t.Errorf("expected one contact, got %+v", contacts.Data)
	}

	// Document upload (multipart), list, and download.
	var doc struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	uploadFile(t, srv.URL+"/api/v1/documents", access, wk.ID, "policy.txt", "hello world", &doc)
	if doc.ID == "" {
		t.Fatal("upload returned no id")
	}
	var docs struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/documents?worker_id="+wk.ID, access, &docs)
	if len(docs.Data) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs.Data))
	}
	// Download returns the exact bytes.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/documents/"+doc.ID+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello world" {
		t.Errorf("download body = %q, want %q", string(body), "hello world")
	}
}

func TestEffectiveDatedAssignments(t *testing.T) {
	srv := testServer(t)

	var reg struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	postJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]any{
		"org_name": fmt.Sprintf("AsOf Co %d", time.Now().UnixNano()),
		"email":    "hr@asof.co", "password": "password123",
	}, &reg)
	access := reg.Token.AccessToken

	var wk struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/workers", access, map[string]any{
		"employee_number": "A-001", "first_name": "As", "last_name": "Of",
	}, &wk)

	// Two positions to move between.
	var eng, mgr struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/positions", access, map[string]any{"title": "Engineer"}, &eng)
	postJSON(t, srv.URL+"/api/v1/positions", access, map[string]any{"title": "Manager"}, &mgr)

	// Initial assignment effective 2023-01-01, then a promotion effective 2024-06-01.
	postJSON(t, srv.URL+"/api/v1/assignments", access, map[string]any{
		"worker_id": wk.ID, "position_id": eng.ID, "effective_date": "2023-01-01",
	}, nil)
	postJSON(t, srv.URL+"/api/v1/assignments", access, map[string]any{
		"worker_id": wk.ID, "position_id": mgr.ID, "effective_date": "2024-06-01",
		"event_type": "promotion", "reason": "growth",
	}, nil)

	// As of 2023-06-01 the worker held the Engineer position…
	var asEng struct {
		PositionID string `json:"position_id"`
	}
	if s := getJSON(t, srv.URL+"/api/v1/assignments/as-of?worker_id="+wk.ID+"&as_of=2023-06-01", access, &asEng); s != http.StatusOK {
		t.Fatalf("as-of 2023 status = %d", s)
	}
	if asEng.PositionID != eng.ID {
		t.Errorf("as of 2023-06-01 expected Engineer position, got %s", asEng.PositionID)
	}

	// …and as of 2024-12-01 the Manager position.
	var asMgr struct {
		PositionID string `json:"position_id"`
	}
	getJSON(t, srv.URL+"/api/v1/assignments/as-of?worker_id="+wk.ID+"&as_of=2024-12-01", access, &asMgr)
	if asMgr.PositionID != mgr.ID {
		t.Errorf("as of 2024-12-01 expected Manager position, got %s", asMgr.PositionID)
	}

	// Before any assignment: 404.
	if s := getJSON(t, srv.URL+"/api/v1/assignments/as-of?worker_id="+wk.ID+"&as_of=2020-01-01", access, nil); s != http.StatusNotFound {
		t.Errorf("as-of before hire = %d, want 404", s)
	}
}

func TestTimeOffApprovalFlow(t *testing.T) {
	srv := testServer(t)

	var reg struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	postJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]any{
		"org_name": fmt.Sprintf("Leave Co %d", time.Now().UnixNano()),
		"email":    "admin@leave.co", "password": "password123",
	}, &reg)
	admin := reg.Token.AccessToken

	// Admin creates a leave type and two workers (employee + manager).
	var lt struct {
		ID string `json:"id"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/time-off/leave-types", admin, map[string]any{"name": "Vacation"}, &lt); s != http.StatusCreated {
		t.Fatalf("create leave type = %d", s)
	}
	var emp, mgr struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/workers", admin, map[string]any{"employee_number": "E-1", "first_name": "Emp", "last_name": "One"}, &emp)
	postJSON(t, srv.URL+"/api/v1/workers", admin, map[string]any{"employee_number": "M-1", "first_name": "Man", "last_name": "Ager"}, &mgr)

	// Give the employee a starting balance by granting via a request that we
	// then observe; here we just assign the manager so approval is required.
	postJSON(t, srv.URL+"/api/v1/assignments", admin, map[string]any{
		"worker_id": emp.ID, "manager_id": mgr.ID, "effective_date": "2024-01-01",
	}, nil)

	// Employee's time-off request (filed by admin on their behalf) is pending.
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/time-off/requests", admin, map[string]any{
		"worker_id": emp.ID, "leave_type_id": lt.ID,
		"start_date": "2026-08-01", "end_date": "2026-08-02", "hours": 16, "reason": "trip",
	}, &req); s != http.StatusCreated {
		t.Fatalf("create time-off = %d", s)
	}
	if req.Status != "pending" {
		t.Errorf("expected pending (has manager), got %q", req.Status)
	}

	// Admin (org:write) sees it in the pending inbox via override and approves.
	var inbox struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/approvals", admin, &inbox)
	if len(inbox.Data) == 0 {
		t.Fatal("admin override inbox is empty")
	}
	var decided struct {
		Status string `json:"status"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/approvals/"+inbox.Data[0].ID+"/decide", admin, map[string]any{"approve": true}, &decided); s != http.StatusOK {
		t.Fatalf("decide = %d", s)
	}
	if decided.Status != "approved" {
		t.Errorf("approval status = %q, want approved", decided.Status)
	}

	// The time-off request is now approved and the balance was debited 16h.
	var reqs struct {
		Data []struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/time-off/requests?worker_id="+emp.ID, admin, &reqs)
	if len(reqs.Data) != 1 || reqs.Data[0].Status != "approved" {
		t.Errorf("request not approved: %+v", reqs.Data)
	}
	var bals struct {
		Data []struct {
			LeaveTypeName string  `json:"leave_type_name"`
			BalanceHours  float64 `json:"balance_hours"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/time-off/balances?worker_id="+emp.ID, admin, &bals)
	var vac float64
	for _, b := range bals.Data {
		if b.LeaveTypeName == "Vacation" {
			vac = b.BalanceHours
		}
	}
	if vac != -16 {
		t.Errorf("balance after 16h debit = %v, want -16", vac)
	}
}

func TestEntityEditAndDelete(t *testing.T) {
	srv := testServer(t)

	var reg struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	postJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]any{
		"org_name": fmt.Sprintf("CRUD Co %d", time.Now().UnixNano()),
		"email":    "admin@crud.co", "password": "password123",
	}, &reg)
	tok := reg.Token.AccessToken

	// Department: create -> update -> delete -> 404.
	var dept struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/departments", tok, map[string]any{"name": "Ops", "code": "OPS"}, &dept)
	var updated struct {
		Name string `json:"name"`
	}
	if s := putJSON(t, srv.URL+"/api/v1/departments/"+dept.ID, tok, map[string]any{"name": "Operations", "code": "OPS"}, &updated); s != http.StatusOK {
		t.Fatalf("update department = %d", s)
	}
	if updated.Name != "Operations" {
		t.Errorf("department name = %q, want Operations", updated.Name)
	}
	if s := deleteReq(t, srv.URL+"/api/v1/departments/"+dept.ID, tok); s != http.StatusNoContent {
		t.Fatalf("delete department = %d", s)
	}
	if s := deleteReq(t, srv.URL+"/api/v1/departments/"+dept.ID, tok); s != http.StatusNotFound {
		t.Errorf("re-delete department = %d, want 404", s)
	}

	// Leave type in use cannot be deleted (409), unused one can (204).
	var lt struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/time-off/leave-types", tok, map[string]any{"name": "Vacation"}, &lt)
	var wk struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/workers", tok, map[string]any{"employee_number": "C-1", "first_name": "C", "last_name": "One"}, &wk)
	postJSON(t, srv.URL+"/api/v1/time-off/requests", tok, map[string]any{
		"worker_id": wk.ID, "leave_type_id": lt.ID,
		"start_date": "2026-08-01", "end_date": "2026-08-01", "hours": 8,
	}, nil)
	if s := deleteReq(t, srv.URL+"/api/v1/time-off/leave-types/"+lt.ID, tok); s != http.StatusConflict {
		t.Errorf("delete in-use leave type = %d, want 409", s)
	}

	var unused struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/time-off/leave-types", tok, map[string]any{"name": "Jury Duty"}, &unused)
	if s := deleteReq(t, srv.URL+"/api/v1/time-off/leave-types/"+unused.ID, tok); s != http.StatusNoContent {
		t.Errorf("delete unused leave type = %d, want 204", s)
	}

	// Worker delete.
	if s := deleteReq(t, srv.URL+"/api/v1/workers/"+wk.ID, tok); s != http.StatusNoContent {
		t.Errorf("delete worker = %d, want 204", s)
	}
	if s := getJSON(t, srv.URL+"/api/v1/workers/"+wk.ID, tok, nil); s != http.StatusNotFound {
		t.Errorf("get deleted worker = %d, want 404", s)
	}
}

func deleteReq(t *testing.T, url, token string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return do(t, req, nil)
}

func TestNotificationsAndUserManagement(t *testing.T) {
	srv := testServer(t)

	var reg struct {
		Organization struct {
			Slug string `json:"slug"`
		} `json:"organization"`
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	postJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]any{
		"org_name": fmt.Sprintf("Notify Co %d", time.Now().UnixNano()),
		"email":    "admin@notify.co", "password": "password123",
	}, &reg)
	admin := reg.Token.AccessToken
	slug := reg.Organization.Slug

	// Two workers: employee reports to manager.
	var emp, mgr struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/workers", admin, map[string]any{"employee_number": "E-1", "first_name": "Ella", "last_name": "Emp"}, &emp)
	postJSON(t, srv.URL+"/api/v1/workers", admin, map[string]any{"employee_number": "M-1", "first_name": "Manny", "last_name": "Mgr"}, &mgr)
	postJSON(t, srv.URL+"/api/v1/assignments", admin, map[string]any{"worker_id": emp.ID, "manager_id": mgr.ID, "effective_date": "2024-01-01"}, nil)

	// Roles + invite logins for both, linked to their workers.
	var rolesResp struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/roles", admin, &rolesResp)
	var employeeRole string
	for _, r := range rolesResp.Data {
		if r.Name == "Employee" {
			employeeRole = r.ID
		}
	}
	if employeeRole == "" {
		t.Fatal("no Employee role found")
	}
	if s := postJSON(t, srv.URL+"/api/v1/users", admin, map[string]any{
		"email": "ella@notify.co", "password": "password123", "worker_id": emp.ID, "role_ids": []string{employeeRole},
	}, nil); s != http.StatusCreated {
		t.Fatalf("invite employee = %d", s)
	}
	postJSON(t, srv.URL+"/api/v1/users", admin, map[string]any{
		"email": "manny@notify.co", "password": "password123", "worker_id": mgr.ID, "role_ids": []string{employeeRole},
	}, nil)

	// Duplicate email is rejected.
	if s := postJSON(t, srv.URL+"/api/v1/users", admin, map[string]any{
		"email": "ella@notify.co", "password": "password123",
	}, nil); s != http.StatusConflict {
		t.Errorf("duplicate invite = %d, want 409", s)
	}

	login := func(email string) string {
		var lr struct {
			Token struct {
				AccessToken string `json:"access_token"`
			} `json:"token"`
		}
		postJSON(t, srv.URL+"/api/v1/auth/login", "", map[string]any{"slug": slug, "email": email, "password": "password123"}, &lr)
		return lr.Token.AccessToken
	}
	ella := login("ella@notify.co")
	manny := login("manny@notify.co")

	// Leave type + Ella requests time off -> Manny (manager) is notified.
	var lt struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/time-off/leave-types", admin, map[string]any{"name": "Vacation"}, &lt)
	postJSON(t, srv.URL+"/api/v1/time-off/requests", ella, map[string]any{
		"leave_type_id": lt.ID, "start_date": "2026-08-01", "end_date": "2026-08-02", "hours": 16,
	}, nil)

	var count struct {
		Unread int `json:"unread"`
	}
	getJSON(t, srv.URL+"/api/v1/notifications/unread-count", manny, &count)
	if count.Unread != 1 {
		t.Errorf("manager unread notifications = %d, want 1", count.Unread)
	}

	// Manny approves -> Ella is notified of the outcome.
	var inbox struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/approvals", manny, &inbox)
	if len(inbox.Data) != 1 {
		t.Fatalf("manager inbox = %d, want 1", len(inbox.Data))
	}
	postJSON(t, srv.URL+"/api/v1/approvals/"+inbox.Data[0].ID+"/decide", manny, map[string]any{"approve": true}, nil)

	var ellaNotifs struct {
		Data []struct {
			Title string `json:"title"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/notifications", ella, &ellaNotifs)
	found := false
	for _, n := range ellaNotifs.Data {
		if n.Title == "Your request was approved" {
			found = true
		}
	}
	if !found {
		t.Errorf("employee not notified of approval; got %+v", ellaNotifs.Data)
	}
}

func TestOnboardingChecklists(t *testing.T) {
	srv := testServer(t)

	var reg struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	postJSON(t, srv.URL+"/api/v1/auth/register", "", map[string]any{
		"org_name": fmt.Sprintf("Board Co %d", time.Now().UnixNano()),
		"email":    "admin@board.co", "password": "password123",
	}, &reg)
	admin := reg.Token.AccessToken

	// Employee + manager, employee reports to manager.
	var emp, mgr struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/workers", admin, map[string]any{"employee_number": "E-1", "first_name": "New", "last_name": "Hire"}, &emp)
	postJSON(t, srv.URL+"/api/v1/workers", admin, map[string]any{"employee_number": "M-1", "first_name": "The", "last_name": "Boss"}, &mgr)
	postJSON(t, srv.URL+"/api/v1/assignments", admin, map[string]any{"worker_id": emp.ID, "manager_id": mgr.ID, "effective_date": "2024-01-01"}, nil)

	// Template with a new-hire task and a manager task.
	var tmpl struct {
		ID string `json:"id"`
	}
	postJSON(t, srv.URL+"/api/v1/checklist-templates", admin, map[string]any{
		"name": "Onboarding", "type": "onboarding",
		"tasks": []map[string]any{
			{"title": "Sign I-9", "assignee": "new_hire", "offset_days": 0},
			{"title": "Provision laptop", "assignee": "manager", "offset_days": 1},
		},
	}, &tmpl)

	// Instantiate for the employee.
	var plan struct {
		ID string `json:"id"`
	}
	if s := postJSON(t, srv.URL+"/api/v1/checklist-plans", admin, map[string]any{
		"worker_id": emp.ID, "template_id": tmpl.ID, "start_date": "2026-08-01",
	}, &plan); s != http.StatusCreated {
		t.Fatalf("create plan = %d", s)
	}

	// Tasks resolved to the right assignees and due dates.
	var full struct {
		TotalTasks int `json:"total_tasks"`
		Tasks      []struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			AssigneeName string `json:"assignee_name"`
			DueDate      string `json:"due_date"`
			Status       string `json:"status"`
		} `json:"tasks"`
	}
	getJSON(t, srv.URL+"/api/v1/checklist-plans/"+plan.ID, admin, &full)
	if len(full.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(full.Tasks))
	}
	byTitle := map[string]struct {
		ID, Assignee, Due string
	}{}
	for _, tk := range full.Tasks {
		byTitle[tk.Title] = struct{ ID, Assignee, Due string }{tk.ID, tk.AssigneeName, tk.DueDate}
	}
	if byTitle["Sign I-9"].Assignee != "New Hire" {
		t.Errorf("new-hire task assignee = %q, want New Hire", byTitle["Sign I-9"].Assignee)
	}
	if byTitle["Provision laptop"].Assignee != "The Boss" {
		t.Errorf("manager task assignee = %q, want The Boss", byTitle["Provision laptop"].Assignee)
	}
	if !strings.HasPrefix(byTitle["Provision laptop"].Due, "2026-08-02") {
		t.Errorf("manager task due = %q, want 2026-08-02 (start+1)", byTitle["Provision laptop"].Due)
	}

	// Complete one task (admin override) and verify progress; plan not yet complete.
	if s := postJSON(t, srv.URL+"/api/v1/checklist-tasks/"+byTitle["Sign I-9"].ID+"/status", admin, map[string]any{"status": "done"}, nil); s != http.StatusNoContent {
		t.Fatalf("complete task = %d", s)
	}
	var plans struct {
		Data []struct {
			DoneTasks int    `json:"done_tasks"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	getJSON(t, srv.URL+"/api/v1/checklist-plans?worker_id="+emp.ID, admin, &plans)
	if plans.Data[0].DoneTasks != 1 || plans.Data[0].Status != "active" {
		t.Errorf("after 1 of 2 done: %+v", plans.Data[0])
	}
}

func putJSON(t *testing.T, url, token string, body any, out any) int {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPut, url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return do(t, req, out)
}

func uploadFile(t *testing.T, url, token, workerID, filename, content string, out any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("worker_id", workerID)
	_ = mw.WriteField("name", filename)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write([]byte(content))
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	if s := do(t, req, out); s != http.StatusCreated {
		t.Fatalf("upload status = %d", s)
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
