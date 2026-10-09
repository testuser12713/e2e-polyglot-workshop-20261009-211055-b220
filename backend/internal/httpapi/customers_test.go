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

	"github.com/jackc/pgx/v5/pgxpool"
)

// newCustomersTestEnv builds the API router plus a pool for schema setup and
// row cleanup. It skips (via newTestEnv) when the databases are absent, so the
// suite still collects on a bare checkout.
func newCustomersTestEnv(t *testing.T) (*testEnv, *pgxpool.Pool) {
	t.Helper()

	env := newTestEnv(t)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	t.Cleanup(pool.Close)

	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return env, pool
}

// customersPostJSON sends a JSON body to the router and returns the recorder.
func customersPostJSON(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// customersGet sends a GET request to the router.
func customersGet(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestCreateCustomerReturns201AndIsRetrievable(t *testing.T) {
	env, pool := newCustomersTestEnv(t)

	email := fmt.Sprintf("kunde-%d@example.com", time.Now().UnixNano())
	rec := customersPostJSON(t, env.handler, "/api/customers", customerCreateRequest{
		Name:  "Anna Beispiel",
		Email: email,
		Phone: "0151 1234567",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/customers: want 201, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var created customerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("create customer body is not JSON: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("created customer has no id: %+v", created)
	}
	if created.Name != "Anna Beispiel" || created.Email != email || created.Phone != "0151 1234567" {
		t.Fatalf("created customer mismatch: %+v", created)
	}

	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, created.ID); err != nil {
			t.Errorf("cleanup customer: %v", err)
		}
	})

	get := customersGet(t, env.handler, fmt.Sprintf("/api/customers/%d", created.ID))
	if get.Code != http.StatusOK {
		t.Fatalf("GET /api/customers/{id}: want 200, got %d (body=%q)", get.Code, get.Body.String())
	}

	var fetched customerResponse
	if err := json.Unmarshal(get.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("get customer body is not JSON: %v", err)
	}
	if fetched != created {
		t.Fatalf("fetched customer %+v differs from created %+v", fetched, created)
	}
}

func TestCreateCustomerMissingNameReturns400(t *testing.T) {
	env, _ := newCustomersTestEnv(t)

	rec := customersPostJSON(t, env.handler, "/api/customers", customerCreateRequest{
		Email: fmt.Sprintf("kunde-%d@example.com", time.Now().UnixNano()),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing name: want 400, got %d (body=%q)", rec.Code, rec.Body.String())
	}
	decodeError(t, rec.Body.Bytes())
}

func TestCreateCustomerInvalidEmailReturns400(t *testing.T) {
	env, _ := newCustomersTestEnv(t)

	for _, invalid := range []string{"not-an-email", "missing@", "@example.com", "a@b", "with space@example.com"} {
		rec := customersPostJSON(t, env.handler, "/api/customers", customerCreateRequest{
			Name:  "Anna Beispiel",
			Email: invalid,
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("invalid e-mail %q: want 400, got %d (body=%q)", invalid, rec.Code, rec.Body.String())
		}
		decodeError(t, rec.Body.Bytes())
	}
}

func TestGetUnknownCustomerReturns404(t *testing.T) {
	env, _ := newCustomersTestEnv(t)

	for _, id := range []string{"999999999", "not-a-number"} {
		rec := customersGet(t, env.handler, "/api/customers/"+id)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET /api/customers/%s: want 404, got %d (body=%q)", id, rec.Code, rec.Body.String())
		}
		decodeError(t, rec.Body.Bytes())
	}
}
