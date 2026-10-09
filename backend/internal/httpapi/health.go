package httpapi

import (
	"context"
	"net/http"
	"time"
)

// healthResponse is the body of GET /api/health. It reports the state of the
// two backends the API depends on.
type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Queue    string `json:"queue"`
}

// health reports whether PostgreSQL and Valkey are reachable. It exercises the
// same clients the feature routes use, so a bound port is not mistaken for a
// working product.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	database := "ok"
	if err := s.store.Ping(ctx); err != nil {
		database = "error"
	}
	queue := "ok"
	if err := s.queue.Ping(ctx); err != nil {
		queue = "error"
	}

	response := healthResponse{Status: "ok", Database: database, Queue: queue}
	status := http.StatusOK
	if database != "ok" || queue != "ok" {
		response.Status = "degraded"
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, response)
}
