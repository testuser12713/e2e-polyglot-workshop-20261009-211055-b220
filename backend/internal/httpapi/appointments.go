package httpapi

import "net/http"

// createAppointment handles POST /api/appointments.
func (s *Server) createAppointment(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, http.MethodPost, "/api/appointments")
}
