package httpapi

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"workshop/api/internal/auth"
	"workshop/api/internal/config"
	"workshop/api/internal/queue"
	"workshop/api/internal/ratelimit"
	"workshop/api/internal/store"
)

// Server carries every dependency a handler needs. One instance is created per
// process and shared by all routes.
type Server struct {
	cfg           *config.Config
	store         *store.Store
	queue         *queue.Client
	loginLimiter  *ratelimit.Limiter
	lookupLimiter *ratelimit.Limiter
}

// NewServer wires the dependencies into a Server.
func NewServer(cfg *config.Config, st *store.Store, q *queue.Client) *Server {
	return &Server{
		cfg:   cfg,
		store: st,
		queue: q,
		// AC-28: at most 10 login attempts per client per minute.
		loginLimiter: ratelimit.New(10, time.Minute),
		// AC-36: at most 20 public lookups per client per minute.
		lookupLimiter: ratelimit.New(20, time.Minute),
	}
}

// Router builds the HTTP handler for the whole API: every route of the shared
// contract, CORS for the configured frontend origin, an auth guard around all
// /workshop/* routes, and the uniform 404/405/500 bodies (AC-19, AC-29).
func Router(cfg *config.Config, st *store.Store, q *queue.Client) http.Handler {
	s := NewServer(cfg, st, q)
	m := newMux()

	// Customer area — public.
	m.handle(http.MethodGet, "/api/health", s.health)
	m.handle(http.MethodPost, "/api/customers", s.createCustomer)
	m.handle(http.MethodGet, "/api/customers/{id}", s.getCustomer)
	m.handle(http.MethodPost, "/api/vehicles", s.createVehicle)
	m.handle(http.MethodPost, "/api/appointments", s.createAppointment)
	m.handle(http.MethodGet, "/api/orders/{number}/status", s.lookupOrderStatus)
	m.handle(http.MethodGet, "/api/orders/{number}/invoice", s.lookupOrderInvoice)
	m.handle(http.MethodPost, "/api/auth/login", s.login)

	// Workshop area — bearer token required on every route (AC-14).
	m.handle(http.MethodGet, "/api/workshop/orders", s.requireAuth(s.listWorkshopOrders))
	m.handle(http.MethodGet, "/api/workshop/orders/{number}", s.requireAuth(s.getWorkshopOrder))
	m.handle(http.MethodPut, "/api/workshop/orders/{number}/positions", s.requireAuth(s.updatePositions))
	m.handle(http.MethodPost, "/api/workshop/orders/{number}/status", s.requireAuth(s.updateOrderStatus))
	m.handle(http.MethodGet, "/api/workshop/dashboard", s.requireAuth(s.dashboard))

	// CORS is outermost so that even a recovered panic keeps its headers.
	return withCORS(cfg.FrontendOrigin, withRecovery(m))
}

// requireAuth rejects a request without a valid bearer token with 401 in the
// uniform error body.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		if _, err := auth.Verify(s.cfg.AuthSecret, token); err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid bearer token")
			return
		}
		next(w, r)
	}
}

// withCORS emits CORS headers for exactly the configured frontend origin and
// for no other (AC-29). A preflight from the allowed origin is answered here;
// a request from any other origin passes through without approval.
func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigin != "" && origin == allowedOrigin {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", allowedOrigin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Add("Vary", "Origin")
			if r.Method == http.MethodOptions {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				requested := r.Header.Get("Access-Control-Request-Headers")
				if requested == "" {
					requested = "Authorization, Content-Type"
				}
				h.Set("Access-Control-Allow-Headers", requested)
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// withRecovery turns an unexpected panic into the uniform 500 body instead of
// letting the connection drop. It sits inside the CORS layer, so the error
// response still carries the CORS headers the browser needs.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("httpapi: recovered panic on %s %s: %v", r.Method, r.URL.Path, rec)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Minimal method-aware router over net/http's standard primitives. It gives
// full control over the 404/405 bodies, which net/http's default mux does not.
// ---------------------------------------------------------------------------

type route struct {
	method   string
	segments []string
	handler  http.HandlerFunc
}

type mux struct {
	routes []route
}

func newMux() *mux { return &mux{} }

func (m *mux) handle(method, pattern string, handler http.HandlerFunc) {
	m.routes = append(m.routes, route{
		method:   method,
		segments: splitPath(pattern),
		handler:  handler,
	})
}

func (m *mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	segments := splitPath(r.URL.Path)
	pathMatched := false

	for _, rt := range m.routes {
		params, ok := matchPath(rt.segments, segments)
		if !ok {
			continue
		}
		pathMatched = true
		if rt.method != r.Method {
			continue
		}
		ctx := r.Context()
		for name, value := range params {
			ctx = context.WithValue(ctx, pathParamKey(name), value)
		}
		rt.handler(w, r.WithContext(ctx))
		return
	}

	if pathMatched {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "resource not found")
}

// splitPath splits a URL path into segments without collapsing a trailing
// slash: /api/x and /api/x/ are deliberately different routes.
func splitPath(path string) []string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return []string{""}
	}
	return strings.Split(path, "/")
}

// matchPath reports whether a request path matches a pattern and returns the
// captured {param} values. Each pattern segment matches exactly one request
// segment; a {name} segment matches any non-empty one.
func matchPath(pattern, path []string) (map[string]string, bool) {
	if len(pattern) != len(path) {
		return nil, false
	}
	params := make(map[string]string)
	for i, seg := range pattern {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			if path[i] == "" {
				return nil, false
			}
			params[seg[1:len(seg)-1]] = path[i]
			continue
		}
		if seg != path[i] {
			return nil, false
		}
	}
	return params, true
}

type contextKey string

func pathParamKey(name string) contextKey {
	return contextKey("pathparam:" + name)
}

// pathParam returns a captured path parameter, or the empty string.
func pathParam(r *http.Request, name string) string {
	value, _ := r.Context().Value(pathParamKey(name)).(string)
	return value
}
