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

	"github.com/jackc/pgx/v5"

	"workshop/api/internal/auth"
)

// workshopOrdersFixture is a self-contained dataset in the real PostgreSQL
// instance: two customers, two vehicles and two orders, one of them with an
// invoice. Every row carries a unique marker so cleanup touches only these.
type workshopOrdersFixture struct {
	env                 *testEnv
	conn                *pgx.Conn
	token               string
	suffix              string
	orderWithInvoice    string
	orderWithoutInvoice string
	plateWithInvoice    string
	plateWithoutInvoice string
	customerName        string
}

// newWorkshopOrdersFixture builds the real router and a seeding connection,
// applies the startup migration and inserts the rows the tests read. It skips
// when PostgreSQL is not configured, so a bare checkout still collects.
func newWorkshopOrdersFixture(t *testing.T) *workshopOrdersFixture {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL must be set for the workshop order integration tests")
	}

	env := newTestEnv(t)
	ctx := context.Background()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })

	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := conn.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	f := &workshopOrdersFixture{
		env:                 env,
		conn:                conn,
		suffix:              fmt.Sprintf("%d", time.Now().UnixNano()),
		customerName:        "Werkstatt Testkunde",
		orderWithInvoice:    "WOB-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		orderWithoutInvoice: "WOA-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		plateWithInvoice:    "WOB-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		plateWithoutInvoice: "WOA-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	token, err := auth.Sign(env.cfg.AuthSecret, auth.Claims{
		Sub:   1,
		Email: "workshop-test@example.test",
		Exp:   time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	f.token = token

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	f.seed(t, ctx)
	return f
}

func (f *workshopOrdersFixture) seed(t *testing.T, ctx context.Context) {
	t.Helper()

	var customerID int64
	if err := f.conn.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		f.customerName, "workshop-orders-"+f.suffix+"@example.test", "012345",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleWith, vehicleWithout int64
	if err := f.conn.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage, customer_id)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		f.plateWithInvoice, "VW", "Golf", 123456, customerID,
	).Scan(&vehicleWith); err != nil {
		t.Fatalf("insert vehicle with invoice: %v", err)
	}
	if err := f.conn.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage, customer_id)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		f.plateWithoutInvoice, "Opel", "Astra", 50000, customerID,
	).Scan(&vehicleWithout); err != nil {
		t.Fatalf("insert vehicle without invoice: %v", err)
	}

	var orderWithID, orderWithoutID int64
	if err := f.conn.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem, labor_minutes)
		 VALUES ($1, $2, $3, 'fertig', $4, $5, $6) RETURNING id`,
		f.orderWithInvoice, customerID, vehicleWith, "2026-10-15", "Bremsbeläge erneuern", 120,
	).Scan(&orderWithID); err != nil {
		t.Fatalf("insert order with invoice: %v", err)
	}
	if err := f.conn.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		 VALUES ($1, $2, $3, 'angefragt', $4, $5) RETURNING id`,
		f.orderWithoutInvoice, customerID, vehicleWithout, "2026-10-20", "Ölwechsel",
	).Scan(&orderWithoutID); err != nil {
		t.Fatalf("insert order without invoice: %v", err)
	}

	if _, err := f.conn.Exec(ctx,
		`INSERT INTO order_items (order_id, description, quantity, unit_price_cents)
		 VALUES ($1, 'Bremsbelag', 2, 4500), ($1, 'Bremsflüssigkeit', 1, 1000)`,
		orderWithID,
	); err != nil {
		t.Fatalf("insert order items: %v", err)
	}

	if _, err := f.conn.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status, changed_at)
		 VALUES ($1, 'bestätigt', now() - interval '2 hour'), ($1, 'fertig', now())`,
		orderWithID,
	); err != nil {
		t.Fatalf("insert history: %v", err)
	}
	if _, err := f.conn.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status) VALUES ($1, 'angefragt')`,
		orderWithoutID,
	); err != nil {
		t.Fatalf("insert history for order without invoice: %v", err)
	}

	if _, err := f.conn.Exec(ctx,
		`INSERT INTO invoices (invoice_number, order_id, issued_at, labor_minutes, net_cents, vat_cents, gross_cents)
		 VALUES ($1, $2, now(), 120, 10000, 1900, 11900)`,
		"RE-2026-"+f.suffix, orderWithID,
	); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := f.conn.Exec(cleanupCtx,
			`DELETE FROM invoices WHERE order_id IN ($1, $2)`, orderWithID, orderWithoutID); err != nil {
			t.Logf("cleanup invoices: %v", err)
		}
		if _, err := f.conn.Exec(cleanupCtx,
			`DELETE FROM order_status_history WHERE order_id IN ($1, $2)`, orderWithID, orderWithoutID); err != nil {
			t.Logf("cleanup history: %v", err)
		}
		if _, err := f.conn.Exec(cleanupCtx,
			`DELETE FROM order_items WHERE order_id IN ($1, $2)`, orderWithID, orderWithoutID); err != nil {
			t.Logf("cleanup items: %v", err)
		}
		if _, err := f.conn.Exec(cleanupCtx,
			`DELETE FROM orders WHERE id IN ($1, $2)`, orderWithID, orderWithoutID); err != nil {
			t.Logf("cleanup orders: %v", err)
		}
		if _, err := f.conn.Exec(cleanupCtx,
			`DELETE FROM vehicles WHERE id IN ($1, $2)`, vehicleWith, vehicleWithout); err != nil {
			t.Logf("cleanup vehicles: %v", err)
		}
		if _, err := f.conn.Exec(cleanupCtx,
			`DELETE FROM customers WHERE id = $1`, customerID); err != nil {
			t.Logf("cleanup customers: %v", err)
		}
	})
}

func (f *workshopOrdersFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	rec := httptest.NewRecorder()
	f.env.handler.ServeHTTP(rec, req)
	return rec
}

func decodeOrderList(t *testing.T, rec *httptest.ResponseRecorder) workshopOrderListBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET orders: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	var body workshopOrderListBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("order list body is not JSON: %v (body=%q)", err, rec.Body.String())
	}
	return body
}

func containsOrder(body workshopOrderListBody, number string) *workshopOrderSummaryBody {
	for i := range body.Orders {
		if body.Orders[i].OrderNumber == number {
			return &body.Orders[i]
		}
	}
	return nil
}

func TestWorkshopOrderListUnfiltered(t *testing.T) {
	f := newWorkshopOrdersFixture(t)

	body := decodeOrderList(t, f.get(t, "/api/workshop/orders"))

	with := containsOrder(body, f.orderWithInvoice)
	without := containsOrder(body, f.orderWithoutInvoice)
	if with == nil || without == nil {
		t.Fatalf("unfiltered list must contain both orders, got %+v", body.Orders)
	}
	if with.Status != "fertig" {
		t.Errorf("order with invoice status: want fertig, got %q", with.Status)
	}
	if with.Plate != f.plateWithInvoice {
		t.Errorf("order with invoice plate: want %q, got %q", f.plateWithInvoice, with.Plate)
	}
	if with.CustomerName != f.customerName {
		t.Errorf("order customer name: want %q, got %q", f.customerName, with.CustomerName)
	}
	if without.Status != "angefragt" {
		t.Errorf("order without invoice status: want angefragt, got %q", without.Status)
	}
}

func TestWorkshopOrderListStatusFilter(t *testing.T) {
	f := newWorkshopOrdersFixture(t)

	body := decodeOrderList(t, f.get(t, "/api/workshop/orders?status=fertig"))

	if containsOrder(body, f.orderWithInvoice) == nil {
		t.Fatalf("status filter fertig must contain the finished order, got %+v", body.Orders)
	}
	if containsOrder(body, f.orderWithoutInvoice) != nil {
		t.Fatalf("status filter fertig must exclude the requested order, got %+v", body.Orders)
	}
	for _, order := range body.Orders {
		if order.Status != "fertig" {
			t.Errorf("status filter returned a %q order", order.Status)
		}
	}
}

func TestWorkshopOrderListPlateSearch(t *testing.T) {
	f := newWorkshopOrdersFixture(t)

	body := decodeOrderList(t, f.get(t, "/api/workshop/orders?plate="+f.plateWithoutInvoice))

	if containsOrder(body, f.orderWithoutInvoice) == nil {
		t.Fatalf("plate search must contain the matching order, got %+v", body.Orders)
	}
	if containsOrder(body, f.orderWithInvoice) != nil {
		t.Fatalf("plate search must exclude the other order, got %+v", body.Orders)
	}
}

func TestWorkshopOrderListCombinedFilterAndSearch(t *testing.T) {
	f := newWorkshopOrdersFixture(t)

	body := decodeOrderList(t,
		f.get(t, "/api/workshop/orders?status=fertig&plate="+f.plateWithInvoice))

	if containsOrder(body, f.orderWithInvoice) == nil {
		t.Fatalf("combined filter must contain the matching order, got %+v", body.Orders)
	}
	if containsOrder(body, f.orderWithoutInvoice) != nil {
		t.Fatalf("combined filter must exclude the other order, got %+v", body.Orders)
	}
}

func decodeOrderDetail(t *testing.T, rec *httptest.ResponseRecorder) workshopOrderDetailBody {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET order detail: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	var body workshopOrderDetailBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("order detail body is not JSON: %v (body=%q)", err, rec.Body.String())
	}
	return body
}

func TestWorkshopOrderDetailWithInvoice(t *testing.T) {
	f := newWorkshopOrdersFixture(t)

	body := decodeOrderDetail(t, f.get(t, "/api/workshop/orders/"+f.orderWithInvoice))

	if body.Order.OrderNumber != f.orderWithInvoice {
		t.Errorf("order number: want %q, got %q", f.orderWithInvoice, body.Order.OrderNumber)
	}
	if body.Order.Status != "fertig" {
		t.Errorf("order status: want fertig, got %q", body.Order.Status)
	}
	if body.Order.Problem != "Bremsbeläge erneuern" {
		t.Errorf("order problem: got %q", body.Order.Problem)
	}
	if body.Order.DesiredDate != "2026-10-15" {
		t.Errorf("order desired date: want 2026-10-15, got %q", body.Order.DesiredDate)
	}
	if body.Customer.Name != f.customerName {
		t.Errorf("customer name: want %q, got %q", f.customerName, body.Customer.Name)
	}
	if body.Vehicle.Plate != f.plateWithInvoice {
		t.Errorf("vehicle plate: want %q, got %q", f.plateWithInvoice, body.Vehicle.Plate)
	}
	if body.Vehicle.Make != "VW" {
		t.Errorf("vehicle make: want VW, got %q", body.Vehicle.Make)
	}
	if body.Positions.LaborMinutes != 120 {
		t.Errorf("labor minutes: want 120, got %d", body.Positions.LaborMinutes)
	}
	if len(body.Positions.Parts) != 2 {
		t.Fatalf("parts: want 2, got %d", len(body.Positions.Parts))
	}
	if len(body.History) != 2 {
		t.Fatalf("history: want 2 entries, got %d", len(body.History))
	}
	if body.History[0].ChangedAt.IsZero() {
		t.Error("history entry must carry changed_at")
	}
	if body.Invoice == nil {
		t.Fatal("invoice: want present, got null")
	}
	if body.Invoice.InvoiceNumber != "RE-2026-"+f.suffix {
		t.Errorf("invoice number: got %q", body.Invoice.InvoiceNumber)
	}
	if len(body.Invoice.Items) != 2 {
		t.Errorf("invoice items: want 2, got %d", len(body.Invoice.Items))
	}
	if body.Invoice.NetCents != 10000 || body.Invoice.VatCents != 1900 || body.Invoice.GrossCents != 11900 {
		t.Errorf("invoice totals: got net=%d vat=%d gross=%d",
			body.Invoice.NetCents, body.Invoice.VatCents, body.Invoice.GrossCents)
	}
}

func TestWorkshopOrderDetailWithoutInvoice(t *testing.T) {
	f := newWorkshopOrdersFixture(t)

	body := decodeOrderDetail(t, f.get(t, "/api/workshop/orders/"+f.orderWithoutInvoice))

	if body.Order.OrderNumber != f.orderWithoutInvoice {
		t.Errorf("order number: want %q, got %q", f.orderWithoutInvoice, body.Order.OrderNumber)
	}
	if body.Order.Status != "angefragt" {
		t.Errorf("order status: want angefragt, got %q", body.Order.Status)
	}
	if body.Invoice != nil {
		t.Errorf("invoice: want null, got %+v", body.Invoice)
	}
	if len(body.Positions.Parts) != 0 {
		t.Errorf("parts: want none, got %d", len(body.Positions.Parts))
	}
	if len(body.History) != 1 {
		t.Errorf("history: want 1 entry, got %d", len(body.History))
	}
	if body.Vehicle.Plate != f.plateWithoutInvoice {
		t.Errorf("vehicle plate: want %q, got %q", f.plateWithoutInvoice, body.Vehicle.Plate)
	}
}

func TestWorkshopOrderDetailUnknownIs404(t *testing.T) {
	f := newWorkshopOrdersFixture(t)

	rec := f.get(t, "/api/workshop/orders/DOES-NOT-EXIST-"+f.suffix)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown order: want 404, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	decodeError(t, rec.Body.Bytes())
}

func TestWorkshopOrdersRequireBearerToken(t *testing.T) {
	f := newWorkshopOrdersFixture(t)

	for _, path := range []string{
		"/api/workshop/orders",
		"/api/workshop/orders/" + f.orderWithInvoice,
	} {
		rec := httptest.NewRecorder()
		f.env.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without token: want 401, got %d", path, rec.Code)
		}
	}
}
