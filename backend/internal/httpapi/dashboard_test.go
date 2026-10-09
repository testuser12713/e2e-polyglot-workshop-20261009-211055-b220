package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"workshop/api/internal/auth"
	"workshop/api/internal/store"
)

// dashboardStore opens the integration database, makes sure the schema exists
// and returns a store the test can seed through. The schema is created here so
// the test never relies on another test having run first.
func dashboardStore(t *testing.T) *store.Store {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL must be set for the dashboard integration test")
	}

	ctx := context.Background()
	st, err := store.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := st.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return st
}

// seedDashboard orders inserts the rows the dashboard must count and returns
// the ids of every seeded order so the test can remove exactly its own rows.
func seedDashboardOrders(t *testing.T, st *store.Store, now time.Time) []int64 {
	t.Helper()
	ctx := context.Background()
	prefix := fmt.Sprintf("DASH-%d", time.Now().UnixNano())
	ids := make([]int64, 0, 6)

	insertOrder := func(suffix, status string) int64 {
		var id int64
		if err := st.Pool.QueryRow(ctx,
			`INSERT INTO orders (order_number, status) VALUES ($1, $2) RETURNING id`,
			prefix+"-"+suffix, status,
		).Scan(&id); err != nil {
			t.Fatalf("seed order %s: %v", suffix, err)
		}
		ids = append(ids, id)
		return id
	}

	// Three open orders: angefragt, bestätigt, in Arbeit.
	insertOrder("open-1", "angefragt")
	insertOrder("open-2", "bestätigt")
	insertOrder("open-3", "in Arbeit")

	// One order finished today (its history reached fertig with today's date).
	todayOrder := insertOrder("done-today", "fertig")
	if err := st.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status, changed_at) VALUES ($1, 'fertig', $2)`,
		todayOrder, now,
	); err != nil {
		t.Fatalf("seed today's history: %v", err)
	}

	// One order finished yesterday: it must not be counted today.
	yesterdayOrder := insertOrder("done-yesterday", "fertig")
	if err := st.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status, changed_at) VALUES ($1, 'fertig', $2)`,
		yesterdayOrder, now.Add(-24*time.Hour),
	); err != nil {
		t.Fatalf("seed yesterday's history: %v", err)
	}

	// Two invoices for two further orders: one this month, one last month.
	thisMonthOrder := insertOrder("invoice-month", "abgeholt")
	if err := st.Exec(ctx,
		`INSERT INTO invoices (invoice_number, order_id, issued_at, gross_cents)
		 VALUES ($1, $2, $3, $4)`,
		prefix+"-RE-1", thisMonthOrder, now, int64(12345),
	); err != nil {
		t.Fatalf("seed this month's invoice: %v", err)
	}

	lastMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour)
	lastMonthOrder := insertOrder("invoice-last-month", "abgeholt")
	if err := st.Exec(ctx,
		`INSERT INTO invoices (invoice_number, order_id, issued_at, gross_cents)
		 VALUES ($1, $2, $3, $4)`,
		prefix+"-RE-2", lastMonthOrder, lastMonthStart, int64(999),
	); err != nil {
		t.Fatalf("seed last month's invoice: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if err := st.Exec(cleanupCtx, `DELETE FROM invoices WHERE order_id = ANY($1)`, ids); err != nil {
			t.Logf("cleanup invoices: %v", err)
		}
		if err := st.Exec(cleanupCtx, `DELETE FROM order_status_history WHERE order_id = ANY($1)`, ids); err != nil {
			t.Logf("cleanup history: %v", err)
		}
		if err := st.Exec(cleanupCtx, `DELETE FROM orders WHERE id = ANY($1)`, ids); err != nil {
			t.Logf("cleanup orders: %v", err)
		}
	})

	return ids
}

func TestDashboardAggregatesOpenFinishedAndRevenue(t *testing.T) {
	env := newTestEnv(t)
	st := dashboardStore(t)

	now := time.Now().UTC()

	baseCtx := context.Background()
	baseline, err := st.DashboardStats(baseCtx, now)
	if err != nil {
		t.Fatalf("baseline dashboard stats: %v", err)
	}

	seedDashboardOrders(t, st, now)

	token, err := auth.Sign(env.cfg.AuthSecret, auth.Claims{
		Sub:   1,
		Email: "mechanic@example.test",
		Exp:   time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/workshop/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/workshop/dashboard: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var body dashboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("dashboard body is not JSON: %v", err)
	}

	if want := baseline.OpenOrders + 3; body.OpenOrders != want {
		t.Errorf("open_orders: want %d, got %d", want, body.OpenOrders)
	}
	if want := baseline.FinishedToday + 1; body.FinishedToday != want {
		t.Errorf("finished_today: want %d, got %d", want, body.FinishedToday)
	}
	if want := baseline.RevenueMonthCents + 12345; body.RevenueMonthCents != want {
		t.Errorf("revenue_month_cents: want %d, got %d", want, body.RevenueMonthCents)
	}
}

func TestDashboardRequiresBearerToken(t *testing.T) {
	env := newTestEnv(t)

	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workshop/dashboard", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("dashboard without token: want 401, got %d", rec.Code)
	}
	decodeError(t, rec.Body.Bytes())
}
