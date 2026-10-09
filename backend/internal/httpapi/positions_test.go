package httpapi

import (
	"bytes"
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
	"workshop/api/internal/config"
	"workshop/api/internal/queue"
	"workshop/api/internal/store"
)

// positionsEnv carries the router plus the store, so a test can both drive the
// endpoint over httptest and verify what was persisted.
type positionsEnv struct {
	handler http.Handler
	cfg     *config.Config
	store   *store.Store
}

// newPositionsEnv builds the real router against the PostgreSQL and Valkey
// instances the environment points at. It applies the startup migration so the
// tests run on a fresh, empty database, and skips on a bare checkout.
func newPositionsEnv(t *testing.T) *positionsEnv {
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

	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := st.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
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
		AuthSecret:     "positions-test-secret",
	}
	return &positionsEnv{handler: Router(cfg, st, q), cfg: cfg, store: st}
}

// createTestOrder inserts an order with a unique number and removes it again
// (and, by cascade, its positions) when the test ends.
func (e *positionsEnv) createTestOrder(t *testing.T) (int64, string) {
	t.Helper()

	number := fmt.Sprintf("AW-TEST-%d", time.Now().UnixNano())
	var id int64
	err := e.store.Pool.QueryRow(context.Background(),
		`INSERT INTO orders (order_number, status) VALUES ($1, 'angefragt') RETURNING id`,
		number).Scan(&id)
	if err != nil {
		t.Fatalf("insert test order: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.store.Pool.Exec(context.Background(), `DELETE FROM orders WHERE id = $1`, id)
	})
	return id, number
}

// validToken mints a bearer token the router accepts.
func (e *positionsEnv) validToken(t *testing.T) string {
	t.Helper()
	token, err := auth.Sign(e.cfg.AuthSecret, auth.Claims{
		Sub:   1,
		Email: "tester@example.test",
		Exp:   time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func (e *positionsEnv) putPositions(t *testing.T, number, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/workshop/orders/"+number+"/positions", bytes.NewBufferString(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func TestUpdatePositionsCreatesAndReplaces(t *testing.T) {
	env := newPositionsEnv(t)
	orderID, number := env.createTestOrder(t)
	token := env.validToken(t)

	first := `{"labor_minutes":90,"parts":[{"description":"Ölwechsel","quantity":1,"unit_price_cents":5000}]}`
	rec := env.putPositions(t, number, token, first)
	if rec.Code != http.StatusOK {
		t.Fatalf("create positions: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var created positionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("create positions: response is not JSON: %v", err)
	}
	if created.LaborMinutes != 90 || len(created.Parts) != 1 || created.Parts[0].Description != "Ölwechsel" {
		t.Fatalf("create positions: unexpected body %+v", created)
	}

	labor, parts, err := env.store.GetOrderPositions(context.Background(), orderID)
	if err != nil {
		t.Fatalf("read positions: %v", err)
	}
	if labor != 90 || len(parts) != 1 || parts[0].UnitPriceCents != 5000 {
		t.Fatalf("create positions: not persisted, got labor=%d parts=%+v", labor, parts)
	}

	second := `{"labor_minutes":120,"parts":[{"description":"Bremsscheibe","quantity":2,"unit_price_cents":8000},{"description":"Bremsbelag","quantity":1,"unit_price_cents":2500}]}`
	rec = env.putPositions(t, number, token, second)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace positions: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	labor, parts, err = env.store.GetOrderPositions(context.Background(), orderID)
	if err != nil {
		t.Fatalf("read positions after replace: %v", err)
	}
	if labor != 120 {
		t.Errorf("replace positions: want labor 120, got %d", labor)
	}
	if len(parts) != 2 {
		t.Fatalf("replace positions: want 2 parts, got %d (%+v)", len(parts), parts)
	}
	if parts[0].Description != "Bremsscheibe" || parts[0].Quantity != 2 || parts[0].UnitPriceCents != 8000 {
		t.Errorf("replace positions: first part wrong: %+v", parts[0])
	}
	if parts[1].Description != "Bremsbelag" {
		t.Errorf("replace positions: second part wrong: %+v", parts[1])
	}
}

func TestUpdatePositionsRejectsInvalidValues(t *testing.T) {
	env := newPositionsEnv(t)
	orderID, number := env.createTestOrder(t)
	token := env.validToken(t)

	cases := []struct {
		name string
		body string
	}{
		{"negative labor minutes", `{"labor_minutes":-1,"parts":[]}`},
		{"quantity below one", `{"labor_minutes":0,"parts":[{"description":"Öl","quantity":0,"unit_price_cents":100}]}`},
		{"negative price", `{"labor_minutes":0,"parts":[{"description":"Öl","quantity":1,"unit_price_cents":-1}]}`},
		{"missing description", `{"labor_minutes":0,"parts":[{"description":"","quantity":1,"unit_price_cents":100}]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := env.putPositions(t, number, token, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d (body=%q)", rec.Code, rec.Body.String())
			}
			decodeError(t, rec.Body.Bytes())
		})
	}

	labor, parts, err := env.store.GetOrderPositions(context.Background(), orderID)
	if err != nil {
		t.Fatalf("read positions: %v", err)
	}
	if labor != 0 || len(parts) != 0 {
		t.Fatalf("rejected request must not change stored positions, got labor=%d parts=%+v", labor, parts)
	}
}

func TestUpdatePositionsUnknownOrderIs404(t *testing.T) {
	env := newPositionsEnv(t)
	token := env.validToken(t)

	rec := env.putPositions(t, "AW-DOES-NOT-EXIST",
		token, `{"labor_minutes":10,"parts":[]}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown order: want 404, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	decodeError(t, rec.Body.Bytes())
}

func TestUpdatePositionsRequiresAuth(t *testing.T) {
	env := newPositionsEnv(t)
	_, number := env.createTestOrder(t)

	rec := env.putPositions(t, number, "", `{"labor_minutes":10,"parts":[]}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("without token: want 401, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	decodeError(t, rec.Body.Bytes())
}
