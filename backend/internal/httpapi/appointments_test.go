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
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

var orderNumberPattern = regexp.MustCompile(`^AU-\d{4}-\d{4}$`)

// appointmentDB opens a dedicated connection for assertions and makes sure the
// startup schema exists, since these tests drive the router directly instead of
// the API's main entry point.
func appointmentDB(t *testing.T, env *testEnv) *pgx.Conn {
	t.Helper()

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, env.cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := conn.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return conn
}

func uniqueEmail(tag string) string {
	return fmt.Sprintf("appointment-%s-%d@example.test", tag, time.Now().UnixNano())
}

func uniquePlate(tag string) string {
	return fmt.Sprintf("AP-%s-%d", tag, time.Now().UnixNano())
}

// cleanupAppointment removes only the rows this test created: the orders that
// reference its own customer/vehicle, plus that customer and vehicle.
func cleanupAppointment(t *testing.T, conn *pgx.Conn, email, plate string) {
	t.Helper()

	ctx := context.Background()
	if _, err := conn.Exec(ctx,
		`DELETE FROM orders
		 WHERE customer_id IN (SELECT id FROM customers WHERE email = $1)
		    OR vehicle_id IN (SELECT id FROM vehicles WHERE plate = $2)`,
		email, plate,
	); err != nil {
		t.Logf("cleanup orders: %v", err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM customers WHERE email = $1`, email); err != nil {
		t.Logf("cleanup customer: %v", err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM vehicles WHERE plate = $1`, plate); err != nil {
		t.Logf("cleanup vehicle: %v", err)
	}
}

func postAppointment(t *testing.T, env *testEnv, body any) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/appointments", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	return rec
}

func validAppointmentBody(email, plate string) map[string]any {
	return map[string]any{
		"customer": map[string]any{
			"name":  "Max Mustermann",
			"email": email,
			"phone": "+49 170 1234567",
		},
		"vehicle": map[string]any{
			"plate":   plate,
			"make":    "VW",
			"model":   "Golf",
			"mileage": 120000,
		},
		"desired_date": "2026-11-03",
		"problem":      "Bremsen quietschen beim Anhalten",
	}
}

// TestCreateAppointmentNewCustomer drives the real route for an unknown
// customer and vehicle and checks the order, its status and its first history
// row landed in the database.
func TestCreateAppointmentNewCustomer(t *testing.T) {
	env := newTestEnv(t)
	conn := appointmentDB(t, env)

	email := uniqueEmail("new")
	plate := uniquePlate("new")
	t.Cleanup(func() { cleanupAppointment(t, conn, email, plate) })

	rec := postAppointment(t, env, validAppointmentBody(email, plate))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/appointments: want 201, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var body appointmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if !orderNumberPattern.MatchString(body.OrderNumber) {
		t.Errorf("order_number %q does not match AU-YYYY-NNNN", body.OrderNumber)
	}
	if body.Status != "angefragt" {
		t.Errorf("status: want angefragt, got %q", body.Status)
	}

	ctx := context.Background()
	var status, problem string
	var desired time.Time
	if err := conn.QueryRow(ctx,
		`SELECT status, problem, desired_date FROM orders WHERE order_number = $1`,
		body.OrderNumber,
	).Scan(&status, &problem, &desired); err != nil {
		t.Fatalf("order not found in database: %v", err)
	}
	if status != "angefragt" {
		t.Errorf("stored status: want angefragt, got %q", status)
	}
	if problem != "Bremsen quietschen beim Anhalten" {
		t.Errorf("stored problem: got %q", problem)
	}
	if desired.Format("2006-01-02") != "2026-11-03" {
		t.Errorf("stored desired_date: got %s", desired.Format("2006-01-02"))
	}

	var historyCount int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM order_status_history h
		 JOIN orders o ON o.id = h.order_id
		 WHERE o.order_number = $1 AND h.status = 'angefragt'`,
		body.OrderNumber,
	).Scan(&historyCount); err != nil {
		t.Fatalf("query history: %v", err)
	}
	if historyCount != 1 {
		t.Errorf("history rows for angefragt: want 1, got %d", historyCount)
	}
}

// TestCreateAppointmentReusesKnownCustomerAndVehicle pre-creates the customer
// and vehicle and checks the route reuses both instead of duplicating them.
func TestCreateAppointmentReusesKnownCustomerAndVehicle(t *testing.T) {
	env := newTestEnv(t)
	conn := appointmentDB(t, env)

	email := uniqueEmail("known")
	plate := uniquePlate("known")
	t.Cleanup(func() { cleanupAppointment(t, conn, email, plate) })

	ctx := context.Background()
	var customerID int64
	if err := conn.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Bekannte Kundin", email, "030 123456",
	).Scan(&customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	var vehicleID int64
	if err := conn.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "Audi", "A3", 50000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}

	rec := postAppointment(t, env, validAppointmentBody(email, plate))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/appointments: want 201, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var body appointmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}

	var customers, vehicles int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM customers WHERE email = $1`, email).Scan(&customers); err != nil {
		t.Fatalf("count customers: %v", err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM vehicles WHERE plate = $1`, plate).Scan(&vehicles); err != nil {
		t.Fatalf("count vehicles: %v", err)
	}
	if customers != 1 {
		t.Errorf("customers with e-mail: want 1 (reused), got %d", customers)
	}
	if vehicles != 1 {
		t.Errorf("vehicles with plate: want 1 (reused), got %d", vehicles)
	}

	var gotCustomer, gotVehicle int64
	if err := conn.QueryRow(ctx,
		`SELECT customer_id, vehicle_id FROM orders WHERE order_number = $1`,
		body.OrderNumber,
	).Scan(&gotCustomer, &gotVehicle); err != nil {
		t.Fatalf("query order links: %v", err)
	}
	if gotCustomer != customerID {
		t.Errorf("order customer_id: want %d, got %d", customerID, gotCustomer)
	}
	if gotVehicle != vehicleID {
		t.Errorf("order vehicle_id: want %d, got %d", vehicleID, gotVehicle)
	}
}

// TestCreateAppointmentRejectsInvalidInput checks every missing or invalid
// field is answered with 400 in the uniform error body.
func TestCreateAppointmentRejectsInvalidInput(t *testing.T) {
	env := newTestEnv(t)

	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing customer name", func(b map[string]any) { b["customer"].(map[string]any)["name"] = "" }},
		{"missing customer email", func(b map[string]any) { b["customer"].(map[string]any)["email"] = "" }},
		{"invalid customer email", func(b map[string]any) { b["customer"].(map[string]any)["email"] = "not-an-email" }},
		{"missing plate", func(b map[string]any) { b["vehicle"].(map[string]any)["plate"] = "" }},
		{"missing desired_date", func(b map[string]any) { b["desired_date"] = "" }},
		{"invalid desired_date", func(b map[string]any) { b["desired_date"] = "03.11.2026" }},
		{"missing problem", func(b map[string]any) { b["problem"] = "" }},
		{"negative mileage", func(b map[string]any) { b["vehicle"].(map[string]any)["mileage"] = -1 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := validAppointmentBody(uniqueEmail("invalid"), uniquePlate("invalid"))
			tc.mutate(body)

			rec := postAppointment(t, env, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("want 400, got %d (body=%q)", rec.Code, rec.Body.String())
			}
			decodeError(t, rec.Body.Bytes())
		})
	}
}

// TestCreateAppointmentRejectsMalformedJSON checks a body that is not JSON at
// all is answered with the uniform 400 body.
func TestCreateAppointmentRejectsMalformedJSON(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/api/appointments", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON: want 400, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	decodeError(t, rec.Body.Bytes())
}
