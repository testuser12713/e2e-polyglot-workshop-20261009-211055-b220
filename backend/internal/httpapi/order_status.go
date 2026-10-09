package httpapi

import "net/http"

// updateOrderStatus handles POST /api/workshop/orders/{number}/status.
func (s *Server) updateOrderStatus(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodPost, "/api/workshop/orders/{number}/status")
}
