package httpapi

import "net/http"

// listWorkshopOrders handles GET /api/workshop/orders.
func (s *Server) listWorkshopOrders(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodGet, "/api/workshop/orders")
}

// getWorkshopOrder handles GET /api/workshop/orders/{number}.
func (s *Server) getWorkshopOrder(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodGet, "/api/workshop/orders/{number}")
}
