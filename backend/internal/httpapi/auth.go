package httpapi

import "net/http"

// login handles POST /api/auth/login.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodPost, "/api/auth/login")
}
