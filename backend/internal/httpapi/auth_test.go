package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"workshop/api/internal/auth"
	"workshop/api/internal/config"
	"workshop/api/internal/queue"
	"workshop/api/internal/store"
)

const (
	testEmployeeEmail    = "workshop.owner@example.test"
	testEmployeeName     = "workshop.owner"
	testEmployeePassword = "correct horse battery staple"
)

// authTestEnv is the real router wired against the real PostgreSQL and Valkey
// instances the environment points at, plus the shared store so a test can
// inspect the rows it wrote.
type authTestEnv struct {
	handler http.Handler
	cfg     *config.Config
	store   *store.Store
}

// newAuthTestEnv builds the integration environment. It applies the same
// migration the API applies at startup and removes only the employee row this
// ticket owns, before and after the test.
func newAuthTestEnv(t *testing.T) *authTestEnv {
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

	sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "migrations", "0001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := st.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	reset := func() {
		if err := st.Exec(context.Background(), `DELETE FROM employees WHERE email = $1`, testEmployeeEmail); err != nil {
			t.Logf("cleanup employees: %v", err)
		}
	}
	reset()
	t.Cleanup(reset)

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
	return &authTestEnv{handler: Router(cfg, st, q), cfg: cfg, store: st}
}

// createTestEmployee inserts the fixture account and returns its bcrypt hash.
func createTestEmployee(t *testing.T, st *store.Store) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(testEmployeePassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash fixture password: %v", err)
	}
	if _, err := st.CreateEmployee(context.Background(), testEmployeeEmail, testEmployeeName, string(hash)); err != nil {
		t.Fatalf("create fixture employee: %v", err)
	}
	return string(hash)
}

// postLogin drives the login route with the given credentials.
func postLogin(t *testing.T, h http.Handler, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLoginSucceedsAndTokenIsAcceptedOnWorkshopRoute(t *testing.T) {
	env := newAuthTestEnv(t)
	hash := createTestEmployee(t, env.store)

	rec := postLogin(t, env.handler, testEmployeeEmail, testEmployeePassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("login with correct credentials: want 200, got %d (body=%q)", rec.Code, rec.Body.String())
	}

	var body loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("login body is not JSON: %v", err)
	}
	if body.Token == "" {
		t.Fatal("login response carries no token")
	}
	if body.Employee.Email != testEmployeeEmail || body.Employee.Name != testEmployeeName {
		t.Fatalf("employee representation: got %+v", body.Employee)
	}

	// AC-30: neither the clear-text password nor its hash may appear in a
	// response, and there is no password field in the employee representation.
	raw := rec.Body.String()
	if strings.Contains(raw, testEmployeePassword) {
		t.Error("login response contains the clear-text password")
	}
	if strings.Contains(raw, hash) {
		t.Error("login response contains the password hash")
	}
	var fields map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
		t.Fatalf("login body is not a JSON object: %v", err)
	}
	employeeJSON, _ := fields["employee"].(map[string]any)
	if _, present := employeeJSON["password"]; present {
		t.Error("employee representation carries a password field")
	}

	// AC-14: the middleware accepts the token issued by the login endpoint on a
	// protected route. The route body is another ticket's stub for now, so the
	// assertion is that auth passed (not 401), not what that handler answers.
	req := httptest.NewRequest(http.MethodGet, "/api/workshop/orders", nil)
	req.Header.Set("Authorization", "Bearer "+body.Token)
	got := httptest.NewRecorder()
	env.handler.ServeHTTP(got, req)
	if got.Code == http.StatusUnauthorized {
		t.Fatalf("token issued by /api/auth/login was rejected on a workshop route (401)")
	}
}

func TestLoginWithWrongPasswordIs401(t *testing.T) {
	env := newAuthTestEnv(t)
	createTestEmployee(t, env.store)

	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)

	wrong := postLogin(t, env.handler, testEmployeeEmail, "not-the-password")
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: want 401, got %d (body=%q)", wrong.Code, wrong.Body.String())
	}
	decodeError(t, wrong.Body.Bytes())

	unknown := postLogin(t, env.handler, "nobody@example.test", testEmployeePassword)
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("unknown e-mail: want 401, got %d (body=%q)", unknown.Code, unknown.Body.String())
	}
	decodeError(t, unknown.Body.Bytes())

	// AC-30: no log line carries the password in clear text.
	if strings.Contains(logs.String(), "not-the-password") || strings.Contains(logs.String(), testEmployeePassword) {
		t.Error("a log line contains the password in clear text")
	}
}

func TestWorkshopRouteRejectsMissingTokenAndAcceptsIssuedToken(t *testing.T) {
	env := newAuthTestEnv(t)
	createTestEmployee(t, env.store)

	without := httptest.NewRecorder()
	env.handler.ServeHTTP(without, httptest.NewRequest(http.MethodGet, "/api/workshop/dashboard", nil))
	if without.Code != http.StatusUnauthorized {
		t.Fatalf("workshop route without token: want 401, got %d", without.Code)
	}
	decodeError(t, without.Body.Bytes())

	login := postLogin(t, env.handler, testEmployeeEmail, testEmployeePassword)
	if login.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d (body=%q)", login.Code, login.Body.String())
	}
	var body loginResponse
	if err := json.Unmarshal(login.Body.Bytes(), &body); err != nil {
		t.Fatalf("login body is not JSON: %v", err)
	}

	with := httptest.NewRecorder()
	withReq := httptest.NewRequest(http.MethodGet, "/api/workshop/dashboard", nil)
	withReq.Header.Set("Authorization", "Bearer "+body.Token)
	env.handler.ServeHTTP(with, withReq)
	if with.Code == http.StatusUnauthorized {
		t.Fatal("workshop route rejected a valid token with 401")
	}
}

func TestLoginIsRateLimitedAfterTenFailures(t *testing.T) {
	env := newAuthTestEnv(t)
	createTestEmployee(t, env.store)

	for i := 0; i < 10; i++ {
		rec := postLogin(t, env.handler, testEmployeeEmail, "wrong-password")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d: want 401, got %d (body=%q)", i+1, rec.Code, rec.Body.String())
		}
	}

	blocked := postLogin(t, env.handler, testEmployeeEmail, "wrong-password")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("11th attempt: want 429, got %d (body=%q)", blocked.Code, blocked.Body.String())
	}
	decodeError(t, blocked.Body.Bytes())

	// The limit is per client: another address is not blocked by this one.
	other := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"email":"`+testEmployeeEmail+`","password":"wrong-password"}`))
	other.RemoteAddr = "198.51.100.7:5555"
	other.Header.Set("Content-Type", "application/json")
	otherRec := httptest.NewRecorder()
	env.handler.ServeHTTP(otherRec, other)
	if otherRec.Code != http.StatusUnauthorized {
		t.Fatalf("second client: want 401 (own budget), got %d", otherRec.Code)
	}
}

func TestEmployeeRowStoresOnlyPasswordHash(t *testing.T) {
	env := newAuthTestEnv(t)
	hash := createTestEmployee(t, env.store)

	employee, err := env.store.FindEmployeeByEmail(context.Background(), testEmployeeEmail)
	if err != nil {
		t.Fatalf("find employee: %v", err)
	}
	if employee.PasswordHash == testEmployeePassword {
		t.Fatal("employee row stores the clear-text password")
	}
	if !strings.HasPrefix(employee.PasswordHash, "$2") {
		t.Fatalf("stored password is not a bcrypt hash: %q", employee.PasswordHash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(employee.PasswordHash), []byte(testEmployeePassword)); err != nil {
		t.Fatalf("stored hash does not verify the password: %v", err)
	}
	if hash == "" {
		t.Fatal("fixture hash empty")
	}
}

func TestSeedFirstEmployeeCreatesOnceAndNeverUpdates(t *testing.T) {
	env := newAuthTestEnv(t)
	ctx := context.Background()

	// The employees table belongs to this ticket: start from a clean slate so
	// the "first employee" condition is deterministic.
	if err := env.store.Exec(ctx, `DELETE FROM employees`); err != nil {
		t.Fatalf("clear employees: %v", err)
	}
	t.Cleanup(func() { _ = env.store.Exec(context.Background(), `DELETE FROM employees`) })

	const email = "seeded.owner@example.test"
	const firstPassword = "first-seed-password"
	const secondPassword = "second-seed-password"

	t.Setenv("SEED_EMPLOYEE_EMAIL", email)
	t.Setenv("SEED_EMPLOYEE_PASSWORD", firstPassword)
	if err := auth.SeedFirstEmployee(ctx, env.store); err != nil {
		t.Fatalf("seed first employee: %v", err)
	}

	seeded, err := env.store.FindEmployeeByEmail(ctx, email)
	if err != nil {
		t.Fatalf("seeded employee missing: %v", err)
	}
	if seeded.PasswordHash == firstPassword {
		t.Fatal("seed stored the clear-text password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(seeded.PasswordHash), []byte(firstPassword)); err != nil {
		t.Fatalf("seeded hash does not verify the configured password: %v", err)
	}

	// A restart with a different configured password must not overwrite the
	// existing account (AC-15: no update on restart).
	t.Setenv("SEED_EMPLOYEE_PASSWORD", secondPassword)
	if err := auth.SeedFirstEmployee(ctx, env.store); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	again, err := env.store.FindEmployeeByEmail(ctx, email)
	if err != nil {
		t.Fatalf("employee missing after second seed: %v", err)
	}
	if again.PasswordHash != seeded.PasswordHash {
		t.Fatal("seed updated the existing employee on restart")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(again.PasswordHash), []byte(firstPassword)); err != nil {
		t.Fatalf("original password no longer verifies after restart: %v", err)
	}
}
