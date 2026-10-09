package httpapi

import "net/http"

// createVehicle handles POST /api/vehicles.
func (s *Server) createVehicle(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodPost, "/api/vehicles")
}
