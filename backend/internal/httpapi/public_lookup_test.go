package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"workshop/api/internal/config"
	"workshop/api/internal/queue"
	"workshop/api/internal/store"
)

// lookupEnv is a fully seeded environment for the public lookup tests: one
// order WITH an invoice and one WITHOUT, sharing the same test database.
type lookupEnv struct {
	handler http.Handler

	plate         string
	orderNumber   string
	invoiceNumber string

	otherPlate     string
	orderNoInvoice string
}

// newLookupEnv provisions its own schema and rows against the real PostgreSQL
// instance the environment points at, and tears them down afterwards. It skips
// when the databases are absent so the suite still collects on a bare checkout.
func newLookupEnv(t *testing.T) *lookupEnv {
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

	// Create the schema this test reads from. The startup migration is
	// idempotent, so applying it here is safe and makes the test independent of
	// any server having run before.
	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := st.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	q, err := queue.New(valkeyURL)
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	cfg := &config.Config{
		DatabaseURL:    databaseURL,
		ValkeyURL:      valkeyURL,
		Port:           "8000",
		HourRateCents:  6500,
		FrontendOrigin: "https://frontend.example.test",
		AuthSecret:     "integration-test-secret",
	}

	tag := time.Now().UnixNano()
	env := &lookupEnv{
		handler:        Router(cfg, st, q),
		plate:          fmt.Sprintf("B-PUB-%d", tag),
		orderNumber:    fmt.Sprintf("AW-PUB-%d", tag),
		invoiceNumber:  fmt.Sprintf("RE-PUB-%d", tag),
		otherPlate:     fmt.Sprintf("B-PUB2-%d", tag),
		orderNoInvoice: fmt.Sprintf("AW-PUB2-%d", tag),
	}

	var orderID, otherOrderID int64
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, id := range []int64{orderID, otherOrderID} {
			if id == 0 {
				continue
			}
			_ = st.Exec(cleanupCtx, `DELETE FROM order_status_history WHERE order_id = $1`, id)
			_ = st.Exec(cleanupCtx, `DELETE FROM order_items WHERE order_id = $1`, id)
			_ = st.Exec(cleanupCtx, `DELETE FROM invoices WHERE order_id = $1`, id)
			_ = st.Exec(cleanupCtx, `DELETE FROM orders WHERE id = $1`, id)
		}
		_ = st.Exec(cleanupCtx, `DELETE FROM vehicles WHERE plate = $1 OR plate = $2`, env.plate, env.otherPlate)
	})

	orderID = seedLookupOrder(t, ctx, st, env.plate, env.orderNumber, "Bremsen quietschen", true)
	otherOrderID = seedLookupOrder(t, ctx, st, env.otherPlate, env.orderNoInvoice, "Ölwechsel fällig", false)
	if _, err := st.Pool.Exec(ctx,
		`INSERT INTO invoices (invoice_number, order_id, labor_minutes, net_cents, vat_cents, gross_cents)
		 VALUES ($1, $2, 90, 12500, 2375, 14875)`,
		env.invoiceNumber, orderID,
	); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	return env
}

// seedLookupOrder inserts a vehicle and an order (with history and items) and
// returns the new order id.
func seedLookupOrder(t *testing.T, ctx context.Context, st *store.Store, plate, number, problem string, withItems bool) int64 {
	t.Helper()

	var vehicleID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, 'VW', 'Golf', 123456) RETURNING id`,
		plate,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}

	var orderID int64
	if err := st.Pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, vehicle_id, status, problem) VALUES ($1, $2, 'fertig', $3) RETURNING id`,
		number, vehicleID, problem,
	).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	for _, status := range []string{"angefragt", "bestätigt"} {
		if _, err := st.Pool.Exec(ctx,
			`INSERT INTO order_status_history (order_id, status) VALUES ($1, $2)`, orderID, status,
		); err != nil {
			t.Fatalf("seed history: %v", err)
		}
	}

	if withItems {
		if _, err := st.Pool.Exec(ctx,
			`INSERT INTO order_items (order_id, description, quantity, unit_price_cents)
			 VALUES ($1, 'Bremsbelag', 2, 4500), ($1, 'Arbeitszeit', 1, 9750)`, orderID,
		); err != nil {
			t.Fatalf("seed items: %v", err)
		}
	}

	return orderID
}

func TestPublicLookupOrderStatus(t *testing.T) {
	env := newLookupEnv(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+env.orderNumber+"/status?plate="+env.plate, nil)
	env.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status with matching plate: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var body lookupStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("status body is not JSON: %v", err)
	}
	if body.OrderNumber != env.orderNumber {
		t.Errorf("order_number: want %q, got %q", env.orderNumber, body.OrderNumber)
	}
	if body.Status != "fertig" {
		t.Errorf("status: want fertig, got %q", body.Status)
	}
	if body.Vehicle.Plate != env.plate {
		t.Errorf("vehicle.plate: want %q, got %q", env.plate, body.Vehicle.Plate)
	}
	if body.Vehicle.Make == "" || body.Vehicle.Model == "" {
		t.Errorf("vehicle make/model must be present, got %+v", body.Vehicle)
	}
	if body.Problem != "Bremsen quietschen" {
		t.Errorf("problem: want the seeded text, got %q", body.Problem)
	}
	if len(body.History) < 2 {
		t.Fatalf("history: want at least 2 entries, got %d", len(body.History))
	}
	if body.History[0].Status != "angefragt" || body.History[len(body.History)-1].Status != "bestätigt" {
		t.Errorf("history order unexpected: %+v", body.History)
	}
	for _, entry := range body.History {
		if entry.ChangedAt == "" {
			t.Errorf("history entry without changed_at: %+v", entry)
		}
	}
}

func TestPublicLookupOrderStatusMismatchingPlateIs404(t *testing.T) {
	env := newLookupEnv(t)

	for _, target := range []string{
		"/api/orders/" + env.orderNumber + "/status?plate=" + env.otherPlate,
		"/api/orders/" + env.orderNumber + "/status?plate=",
		"/api/orders/AW-DOES-NOT-EXIST/status?plate=" + env.plate,
	} {
		rec := httptest.NewRecorder()
		env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d (body=%q)", target, rec.Code, rec.Body.String())
			continue
		}
		decodeError(t, rec.Body.Bytes())
	}
}

func TestPublicLookupOrderInvoice(t *testing.T) {
	env := newLookupEnv(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+env.orderNumber+"/invoice?plate="+env.plate, nil)
	env.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("invoice with matching plate: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var body lookupInvoiceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invoice body is not JSON: %v", err)
	}
	if body.InvoiceNumber != env.invoiceNumber {
		t.Errorf("invoice_number: want %q, got %q", env.invoiceNumber, body.InvoiceNumber)
	}
	if body.IssuedAt == "" {
		t.Errorf("issued_at must be present")
	}
	if body.LaborMinutes != 90 {
		t.Errorf("labor_minutes: want 90, got %d", body.LaborMinutes)
	}
	if body.NetCents != 12500 || body.VatCents != 2375 || body.GrossCents != 14875 {
		t.Errorf("cents: want net 12500 / vat 2375 / gross 14875, got %d / %d / %d",
			body.NetCents, body.VatCents, body.GrossCents)
	}
	if len(body.Items) != 2 {
		t.Fatalf("items: want 2, got %d", len(body.Items))
	}
	if body.Items[0].Description == "" || body.Items[0].Quantity == 0 || body.Items[0].UnitPriceCents == 0 {
		t.Errorf("first item is incomplete: %+v", body.Items[0])
	}
}

func TestPublicLookupOrderInvoiceNotFound(t *testing.T) {
	env := newLookupEnv(t)

	// Wrong plate on an order that has an invoice.
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/orders/"+env.orderNumber+"/invoice?plate="+env.otherPlate, nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("invoice with mismatching plate: want 404, got %d", rec.Code)
	} else {
		decodeError(t, rec.Body.Bytes())
	}

	// An order that exists (matching plate) but has no invoice yet.
	rec = httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/orders/"+env.orderNoInvoice+"/invoice?plate="+env.otherPlate, nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("invoice for order without invoice: want 404, got %d (body=%q)", rec.Code, rec.Body.String())
	} else {
		decodeError(t, rec.Body.Bytes())
	}
}

func TestPublicLookupRateLimit(t *testing.T) {
	env := newLookupEnv(t)

	target := "/api/orders/" + env.orderNumber + "/status?plate=" + env.plate
	for i := 0; i < 20; i++ {
		rec := httptest.NewRecorder()
		env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d: limit reached too early", i+1)
		}
	}

	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("21st request: want 429, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	decodeError(t, rec.Body.Bytes())
	if strings.Contains(rec.Body.String(), env.plate) {
		t.Errorf("429 body must not leak customer data, got %q", rec.Body.String())
	}

	// The invoice endpoint shares the same per-client budget and must also be
	// rate limited without leaking data.
	rec = httptest.NewRecorder()
	env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/orders/"+env.orderNumber+"/invoice?plate="+env.plate, nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("invoice after the status budget is spent: want 429, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	decodeError(t, rec.Body.Bytes())
	if strings.Contains(rec.Body.String(), env.invoiceNumber) {
		t.Errorf("429 body must not leak invoice data, got %q", rec.Body.String())
	}
}
