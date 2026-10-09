package httpapi

import "net/http"

// lookupOrderStatus handles GET /api/orders/{number}/status.
func (s *Server) lookupOrderStatus(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodGet, "/api/orders/{number}/status")
}

// lookupOrderInvoice handles GET /api/orders/{number}/invoice.
func (s *Server) lookupOrderInvoice(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodGet, "/api/orders/{number}/invoice")
}
