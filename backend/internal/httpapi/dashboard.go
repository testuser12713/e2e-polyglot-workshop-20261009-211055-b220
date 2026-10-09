package httpapi

import (
	"context"
	"log"
	"net/http"
	"time"
)

// dashboardResponse is the body of GET /api/workshop/dashboard (AC-18).
type dashboardResponse struct {
	OpenOrders        int64 `json:"open_orders"`
	FinishedToday     int64 `json:"finished_today"`
	RevenueMonthCents int64 `json:"revenue_month_cents"`
}

// dashboard handles GET /api/workshop/dashboard. It aggregates the workshop
// figures in SQL and answers with whole cents.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	stats, err := s.store.DashboardStats(ctx, time.Now())
	if err != nil {
		log.Printf("httpapi: dashboard: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		OpenOrders:        stats.OpenOrders,
		FinishedToday:     stats.FinishedToday,
		RevenueMonthCents: stats.RevenueMonthCents,
	})
}
