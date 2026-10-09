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

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"workshop/api/internal/auth"
	"workshop/api/internal/store"
)

// setupOrder creates an order in the given status against the real PostgreSQL
// and returns the connected environment plus a helper connection for direct
// assertions. Only the orders (and via cascade their history) rows this test
// creates are touched.
func setupOrder(t *testing.T, status string) (*testEnv, context.Context, *pgx.Conn, int64, string) {
	t.Helper()

	env := newTestEnv(t)
	databaseURL := os.Getenv("DATABASE_URL")

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })

	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := conn.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	number := fmt.Sprintf("AW-TEST-%d", time.Now().UnixNano())
	var id int64
	if err := conn.QueryRow(ctx,
		`INSERT INTO orders (order_number, status, problem) VALUES ($1, $2, 'Testproblem') RETURNING id`,
		number, status).Scan(&id); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), `DELETE FROM orders WHERE id = $1`, id)
	})
	return env, ctx, conn, id, number
}

// testToken signs a valid workshop bearer token for the given secret.
func testToken(t *testing.T, secret string) string {
	t.Helper()
	token, err := auth.Sign(secret, auth.Claims{
		Sub:   1,
		Email: "tester@example.test",
		Exp:   time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

// postStatus sends POST /api/workshop/orders/{number}/status with the bearer
// token and records the response.
func postStatus(t *testing.T, env *testEnv, number, status, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(fmt.Sprintf(`{"status":%q}`, status))
	req := httptest.NewRequest(http.MethodPost, "/api/workshop/orders/"+number+"/status", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	return rec
}

// TestUpdateOrderStatusAcceptsSingleStepAndRecordsHistory covers AC-05 (a
// legal single step answers 200) and AC-06 (the change is stored with a UTC
// timestamp and can be read back).
func TestUpdateOrderStatusAcceptsSingleStepAndRecordsHistory(t *testing.T) {
	env, ctx, conn, id, number := setupOrder(t, "angefragt")
	token := testToken(t, env.cfg.AuthSecret)

	rec := postStatus(t, env, number, "bestätigt", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("legal step: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var body struct {
		Status  string                    `json:"status"`
		History []store.OrderStatusChange `json:"history"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the expected JSON: %v (body=%q)", err, rec.Body.String())
	}
	if body.Status != "bestätigt" {
		t.Errorf("response status: want bestätigt, got %q", body.Status)
	}
	if len(body.History) != 1 || body.History[0].Status != "bestätigt" {
		t.Fatalf("response history: want exactly one bestätigt entry, got %+v", body.History)
	}
	if body.History[0].ChangedAt.IsZero() {
		t.Fatal("history entry is missing changed_at")
	}
	if body.History[0].ChangedAt.Location() != time.UTC {
		t.Errorf("changed_at must be UTC, got location %v", body.History[0].ChangedAt.Location())
	}
	if drift := time.Since(body.History[0].ChangedAt); drift < 0 || drift > 5*time.Minute {
		t.Errorf("changed_at is not a current timestamp: %v", body.History[0].ChangedAt)
	}

	var dbStatus string
	if err := conn.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, id).Scan(&dbStatus); err != nil {
		t.Fatalf("read order status: %v", err)
	}
	if dbStatus != "bestätigt" {
		t.Errorf("stored order status: want bestätigt, got %q", dbStatus)
	}

	var historyRows int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM order_status_history WHERE order_id = $1`, id).Scan(&historyRows); err != nil {
		t.Fatalf("count history rows: %v", err)
	}
	if historyRows != 1 {
		t.Errorf("stored history rows: want 1, got %d", historyRows)
	}
}

// TestUpdateOrderStatusRejectsIllegalTransitions covers AC-05: a jump of two
// steps and a repeat of the current status are both answered with 409 in the
// uniform error body, and neither changes the stored status.
func TestUpdateOrderStatusRejectsIllegalTransitions(t *testing.T) {
	env, ctx, conn, id, number := setupOrder(t, "angefragt")
	token := testToken(t, env.cfg.AuthSecret)

	for _, illegal := range []string{"in Arbeit", "angefragt"} {
		rec := postStatus(t, env, number, illegal, token)
		if rec.Code != http.StatusConflict {
			t.Errorf("status %q: want 409, got %d (body=%q)", illegal, rec.Code, rec.Body.String())
			continue
		}
		decodeError(t, rec.Body.Bytes())
	}

	var dbStatus string
	if err := conn.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, id).Scan(&dbStatus); err != nil {
		t.Fatalf("read order status: %v", err)
	}
	if dbStatus != "angefragt" {
		t.Errorf("rejected transitions must not move the order, got %q", dbStatus)
	}

	var historyRows int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM order_status_history WHERE order_id = $1`, id).Scan(&historyRows); err != nil {
		t.Fatalf("count history rows: %v", err)
	}
	if historyRows != 0 {
		t.Errorf("rejected transitions must not write history, got %d rows", historyRows)
	}
}

// TestUpdateOrderStatusUnknownOrderIs404 covers the 404 branch of the contract.
func TestUpdateOrderStatusUnknownOrderIs404(t *testing.T) {
	env := newTestEnv(t)
	token := testToken(t, env.cfg.AuthSecret)

	rec := postStatus(t, env, "AW-TEST-DOES-NOT-EXIST", "bestätigt", token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown order: want 404, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	decodeError(t, rec.Body.Bytes())
}

// TestUpdateOrderStatusToFinishedEnqueuesExactlyOneInvoiceMessage covers AC-07:
// the transition to fertig pushes exactly one {"order_id": <id>} message onto
// the Valkey list invoices.
func TestUpdateOrderStatusToFinishedEnqueuesExactlyOneInvoiceMessage(t *testing.T) {
	env, ctx, _, id, number := setupOrder(t, "in Arbeit")
	token := testToken(t, env.cfg.AuthSecret)

	valkeyURL := os.Getenv("VALKEY_URL")
	opt, err := redis.ParseURL(valkeyURL)
	if err != nil {
		t.Fatalf("parse VALKEY_URL: %v", err)
	}
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	if err := rdb.Del(ctx, invoiceQueueName).Err(); err != nil {
		t.Fatalf("clear invoices list: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Del(context.Background(), invoiceQueueName).Err() })

	rec := postStatus(t, env, number, "fertig", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("finish: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	entries, err := rdb.LRange(ctx, invoiceQueueName, 0, -1).Result()
	if err != nil {
		t.Fatalf("read invoices list: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("invoices list: want exactly 1 message, got %d (%v)", len(entries), entries)
	}

	want := fmt.Sprintf(`{"order_id":%d}`, id)
	if entries[0] != want {
		t.Errorf("invoice message: want %s, got %s", want, entries[0])
	}
}

// TestUpdateOrderStatusRequiresAuthentication guards the workshop route: no
// bearer token is a 401, never a status change.
func TestUpdateOrderStatusRequiresAuthentication(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/api/workshop/orders/AW-TEST-X/status",
		strings.NewReader(`{"status":"bestätigt"}`))
	rec := httptest.NewRecorder()
	env.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("without token: want 401, got %d", rec.Code)
	}
	decodeError(t, rec.Body.Bytes())
}
