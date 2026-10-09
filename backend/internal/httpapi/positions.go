package httpapi

import "net/http"

// updatePositions handles PUT /api/workshop/orders/{number}/positions.
func (s *Server) updatePositions(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodPut, "/api/workshop/orders/{number}/positions")
}
