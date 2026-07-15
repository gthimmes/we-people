// Command seed populates a fresh database with a demo organization: an admin
// user, departments, locations, positions, workers, and a reporting hierarchy.
//
// It is idempotent-ish: if the demo org already exists it exits without error.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/approvals"
	"github.com/gthimmes/we-people/backend/internal/audit"
	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/config"
	"github.com/gthimmes/we-people/backend/internal/database"
	"github.com/gthimmes/we-people/backend/internal/iam"
	"github.com/gthimmes/we-people/backend/internal/onboarding"
	"github.com/gthimmes/we-people/backend/internal/org"
	"github.com/gthimmes/we-people/backend/internal/orgstructure"
	"github.com/gthimmes/we-people/backend/internal/timeoff"
	"github.com/gthimmes/we-people/backend/internal/worker"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	pool := conn.Pool

	tokens := auth.NewTokenIssuer(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	auditLog := audit.NewLogger(pool)
	iamSvc := iam.NewService(iam.NewStore(pool), org.NewStore(pool), tokens)
	workerSvc := worker.NewService(worker.NewStore(pool), auditLog)
	structSvc := orgstructure.NewService(orgstructure.NewStore(pool), auditLog)
	approvalSvc := approvals.NewService(approvals.NewStore(pool), auditLog)
	timeoffSvc := timeoff.NewService(timeoff.NewStore(pool), approvalSvc, auditLog)
	approvalSvc.RegisterFinalizer(timeoff.RequestType, timeoffSvc)

	reg, err := iamSvc.Register(ctx, "Acme Corp", "admin@acme.test", "password123")
	if errors.Is(err, iam.ErrSlugTaken) {
		fmt.Println("demo org already seeded — nothing to do")
		return nil
	}
	if err != nil {
		return fmt.Errorf("register demo org: %w", err)
	}
	orgID := reg.Org.ID
	actor := reg.User.ID
	fmt.Printf("created org %q (slug=%s) admin=admin@acme.test / password123\n", reg.Org.Name, reg.Org.Slug)

	// Locations
	hq, err := structSvc.CreateLocation(ctx, orgID, actor, orgstructure.Location{
		Name: "HQ", City: "Austin", Region: "TX", Country: "US", Timezone: "America/Chicago",
	})
	if err != nil {
		return err
	}

	// Departments
	eng, err := structSvc.CreateDepartment(ctx, orgID, actor, "Engineering", "ENG", nil, "CC-100")
	if err != nil {
		return err
	}
	sales, err := structSvc.CreateDepartment(ctx, orgID, actor, "Sales", "SAL", nil, "CC-200")
	if err != nil {
		return err
	}

	// Positions
	ceoPos, _ := structSvc.CreatePosition(ctx, orgID, actor, orgstructure.Position{Title: "Chief Executive Officer", LocationID: &hq.ID})
	engMgrPos, _ := structSvc.CreatePosition(ctx, orgID, actor, orgstructure.Position{Title: "Engineering Manager", DepartmentID: &eng.ID, LocationID: &hq.ID})
	engPos1, _ := structSvc.CreatePosition(ctx, orgID, actor, orgstructure.Position{Title: "Software Engineer", DepartmentID: &eng.ID, LocationID: &hq.ID})
	engPos2, _ := structSvc.CreatePosition(ctx, orgID, actor, orgstructure.Position{Title: "Software Engineer", DepartmentID: &eng.ID, LocationID: &hq.ID})
	salesMgrPos, _ := structSvc.CreatePosition(ctx, orgID, actor, orgstructure.Position{Title: "Sales Manager", DepartmentID: &sales.ID, LocationID: &hq.ID})
	salesPos1, _ := structSvc.CreatePosition(ctx, orgID, actor, orgstructure.Position{Title: "Account Executive", DepartmentID: &sales.ID, LocationID: &hq.ID})

	// Workers
	hire := func(num, first, last, email string) worker.Worker {
		wk, e := workerSvc.Create(ctx, orgID, actor, worker.CreateInput{
			EmployeeNumber: num, FirstName: first, LastName: last,
			WorkEmail: email, Status: "active", HireDate: ptrDate("2023-01-15"),
		})
		if e != nil {
			panic(e)
		}
		return wk
	}
	dana := hire("E-001", "Dana", "Reyes", "dana@acme.test")
	sam := hire("E-002", "Sam", "Lee", "sam@acme.test")
	priya := hire("E-003", "Priya", "Patel", "priya@acme.test")
	marcus := hire("E-004", "Marcus", "Chen", "marcus@acme.test")
	jordan := hire("E-005", "Jordan", "Kim", "jordan@acme.test")
	alex := hire("E-006", "Alex", "Rivera", "alex@acme.test")

	// Assignments (build the reporting hierarchy), effective at hire so
	// effective-dated "as of date" queries have meaningful history.
	hireDate := ptrDate("2023-01-15")
	assign := func(wk worker.Worker, posID uuid.UUID, mgr *worker.Worker) {
		in := orgstructure.AssignInput{WorkerID: wk.ID, PositionID: &posID, EffectiveDate: *hireDate}
		if mgr != nil {
			in.ManagerID = &mgr.ID
		}
		if _, e := structSvc.Assign(ctx, orgID, actor, in); e != nil {
			panic(e)
		}
	}
	assign(dana, ceoPos.ID, nil)
	assign(sam, engMgrPos.ID, &dana)
	assign(priya, engPos1.ID, &sam)
	assign(marcus, engPos2.ID, &sam)
	assign(jordan, salesMgrPos.ID, &dana)
	assign(alex, salesPos1.ID, &jordan)

	fmt.Println("seeded 6 workers, 2 departments, 1 location, 6 positions, and the org chart")

	// Linked logins so the manager-approval flow is demoable end to end.
	if _, err := iamSvc.CreateWorkerUser(ctx, orgID, sam.ID, "sam@acme.test", "password123", "Employee"); err != nil {
		return fmt.Errorf("create sam user: %w", err)
	}
	if _, err := iamSvc.CreateWorkerUser(ctx, orgID, priya.ID, "priya@acme.test", "password123", "Employee"); err != nil {
		return fmt.Errorf("create priya user: %w", err)
	}
	fmt.Println("created logins: sam@acme.test (manager), priya@acme.test (reports to Sam) / password123")

	// Leave types (with accrual config) and starting balances.
	carryover := 80.0
	vacation, err := timeoffSvc.CreateLeaveType(ctx, orgID, timeoff.LeaveType{
		Name: "Vacation", IsPaid: true, AccrualEnabled: true,
		AccrualAnnualHours: 120, MaxBalanceHours: 240, CarryoverMaxHours: &carryover,
	})
	if err != nil {
		return err
	}
	sick, err := timeoffSvc.CreateLeaveType(ctx, orgID, timeoff.LeaveType{
		Name: "Sick", IsPaid: true, AccrualEnabled: true, AccrualAnnualHours: 48, MaxBalanceHours: 80,
	})
	if err != nil {
		return err
	}
	if _, err := timeoffSvc.EnsureLeaveType(ctx, orgID, "Personal", true); err != nil {
		return err
	}
	for _, wk := range []worker.Worker{dana, sam, priya, marcus, jordan, alex} {
		if err := timeoffSvc.GrantBalance(ctx, orgID, wk.ID, vacation.ID, 120); err != nil {
			return err
		}
		if err := timeoffSvc.GrantBalance(ctx, orgID, wk.ID, sick.ID, 40); err != nil {
			return err
		}
	}
	fmt.Println("seeded 3 leave types and starting balances (120h vacation, 40h sick each)")

	// Onboarding template + a live plan for Priya so the module has demo data.
	onboardSvc := onboarding.NewService(onboarding.NewStore(pool), auditLog)
	tmpl, err := onboardSvc.CreateTemplate(ctx, orgID, actor, onboarding.TemplateInput{
		Name: "New Hire Onboarding", Type: "onboarding",
		Description: "Standard first-week checklist",
		Tasks: []onboarding.TemplateTask{
			{Title: "Sign offer letter & I-9", Assignee: "new_hire", OffsetDays: 0},
			{Title: "Set up laptop & accounts", Assignee: "manager", OffsetDays: 0},
			{Title: "Complete W-4 and direct deposit", Assignee: "new_hire", OffsetDays: 1},
			{Title: "Meet the team / 1:1 with manager", Assignee: "manager", OffsetDays: 2},
			{Title: "Read the employee handbook", Assignee: "new_hire", OffsetDays: 3},
		},
	})
	if err != nil {
		return fmt.Errorf("seed onboarding template: %w", err)
	}
	if _, err := onboardSvc.CreateTemplate(ctx, orgID, actor, onboarding.TemplateInput{
		Name: "Offboarding", Type: "offboarding", Description: "Departure checklist",
		Tasks: []onboarding.TemplateTask{
			{Title: "Revoke system access", Assignee: "manager", OffsetDays: 0},
			{Title: "Return laptop & equipment", Assignee: "new_hire", OffsetDays: 0},
			{Title: "Exit interview", Assignee: "manager", OffsetDays: 1},
		},
	}); err != nil {
		return fmt.Errorf("seed offboarding template: %w", err)
	}
	if _, err := onboardSvc.CreatePlan(ctx, orgID, actor, priya.ID, tmpl.ID, *ptrDate("2026-07-13")); err != nil {
		return fmt.Errorf("seed onboarding plan: %w", err)
	}
	fmt.Println("seeded onboarding + offboarding templates and a live plan for Priya")
	return nil
}

func ptrDate(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &t
}
