package httpapi

import "net/http"

// dashboard handles GET /api/workshop/dashboard.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodGet, "/api/workshop/dashboard")
}
