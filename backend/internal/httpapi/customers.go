package httpapi

import "net/http"

// createCustomer handles POST /api/customers.
func (s *Server) createCustomer(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodPost, "/api/customers")
}

// getCustomer handles GET /api/customers/{id}.
func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodGet, "/api/customers/{id}")
}
