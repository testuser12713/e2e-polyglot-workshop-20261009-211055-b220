package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"workshop/api/internal/config"
	"workshop/api/internal/queue"
	"workshop/api/internal/store"
)

type testEnv struct {
	handler http.Handler
	cfg     *config.Config
}

// newTestEnv builds the real router against the real PostgreSQL and Valkey
// instances the environment points at. The tests skip when those are absent so
// the suite still collects on a bare checkout.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	valkeyURL := os.Getenv("VALKEY_URL")
	if databaseURL == "" || valkeyURL == "" {
		t.Skip("DATABASE_URL and VALKEY_URL must be set for the API integration tests")
	}

	ctx := context.Background()
	st, err := store.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(st.Close)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := st.Ping(pingCtx); err != nil {
		t.Fatalf("postgres is not reachable: %v", err)
	}

	q, err := queue.New(valkeyURL)
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })
	if err := q.Ping(ctx); err != nil {
		t.Fatalf("valkey is not reachable: %v", err)
	}

	cfg := &config.Config{
		DatabaseURL:    databaseURL,
		ValkeyURL:      valkeyURL,
		Port:           "8000",
		HourRateCents:  6500,
		FrontendOrigin: "https://frontend.example.test",
		AuthSecret:     "integration-test-secret",
	}
	return &testEnv{handler: Router(cfg, st, q), cfg: cfg}
}

func decodeError(t *testing.T, body []byte) errorBody {
	t.Helper()
	var parsed errorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("error body is not the uniform JSON shape: %v (body=%q)", err, string(body))
	}
	if parsed.Code == "" || parsed.Message == "" {
		t.Fatalf("error body must carry code and message, got %+v", parsed)
	}
	return parsed
}

func TestHealthReportsDatabaseAndQueue(t *testing.T) {
	env := newTestEnv(t)

	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/health: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Errorf("GET /api/health: want a JSON content type, got %q", ct)
	}

	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body is not JSON: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("health status: want ok, got %q", body.Status)
	}
	if body.Database != "ok" {
		t.Errorf("health database: want ok, got %q", body.Database)
	}
	if body.Queue != "ok" {
		t.Errorf("health queue: want ok, got %q", body.Queue)
	}
}

func TestUnknownRouteIs404WithUniformBody(t *testing.T) {
	env := newTestEnv(t)

	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown route: want 404, got %d", rec.Code)
	}
	decodeError(t, rec.Body.Bytes())
}

func TestWrongMethodIs405WithUniformBody(t *testing.T) {
	env := newTestEnv(t)

	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/health", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method: want 405, got %d", rec.Code)
	}
	decodeError(t, rec.Body.Bytes())
}

func TestTrailingSlashIsNotASecondSpelling(t *testing.T) {
	env := newTestEnv(t)

	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health/", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("trailing slash: want 404, got %d", rec.Code)
	}
}

func TestCORSOnlyAllowsConfiguredOrigin(t *testing.T) {
	env := newTestEnv(t)

	allowed := httptest.NewRecorder()
	allowedReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	allowedReq.Header.Set("Origin", env.cfg.FrontendOrigin)
	env.handler.ServeHTTP(allowed, allowedReq)
	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != env.cfg.FrontendOrigin {
		t.Errorf("allowed origin: want %q, got %q", env.cfg.FrontendOrigin, got)
	}

	other := httptest.NewRecorder()
	otherReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	otherReq.Header.Set("Origin", "https://evil.example.test")
	env.handler.ServeHTTP(other, otherReq)
	if got := other.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("foreign origin must not be approved, got %q", got)
	}

	preflight := httptest.NewRecorder()
	preflightReq := httptest.NewRequest(http.MethodOptions, "/api/workshop/orders", nil)
	preflightReq.Header.Set("Origin", env.cfg.FrontendOrigin)
	preflightReq.Header.Set("Access-Control-Request-Method", http.MethodGet)
	env.handler.ServeHTTP(preflight, preflightReq)
	if preflight.Code != http.StatusNoContent {
		t.Errorf("preflight: want 204, got %d", preflight.Code)
	}
	if got := preflight.Header().Get("Access-Control-Allow-Origin"); got != env.cfg.FrontendOrigin {
		t.Errorf("preflight origin: want %q, got %q", env.cfg.FrontendOrigin, got)
	}
}

func TestWorkshopRoutesRequireBearerToken(t *testing.T) {
	env := newTestEnv(t)

	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/workshop/dashboard", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("workshop route without token: want 401, got %d", rec.Code)
	}
	decodeError(t, rec.Body.Bytes())

	bad := httptest.NewRecorder()
	badReq := httptest.NewRequest(http.MethodGet, "/api/workshop/dashboard", nil)
	badReq.Header.Set("Authorization", "Bearer not-a-token")
	env.handler.ServeHTTP(bad, badReq)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("workshop route with a bad token: want 401, got %d", bad.Code)
	}
}

// TestInitMigrationIsValid applies the startup migration inside a transaction
// that is rolled back, so it proves the SQL runs and creates the schema
// without leaving tables behind for parallel tests.
func TestInitMigrationIsValid(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL must be set for the migration test")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	for _, table := range []string{
		"customers", "vehicles", "orders", "order_items",
		"order_status_history", "invoices", "outbox", "employees", "counters",
	} {
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			 WHERE table_schema = 'public' AND table_name = $1)`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("migration did not create table %s", table)
		}
	}
}
