package store

import (
	"context"
	"fmt"
	"time"
)

// DashboardStats is the aggregate shown by the workshop dashboard. Every value
// is a whole number of cents or a row count: open orders, orders that reached
// "fertig" today (UTC) and the gross revenue of the current month.
type DashboardStats struct {
	OpenOrders        int64
	FinishedToday     int64
	RevenueMonthCents int64
}

// DashboardStats computes the three aggregates of AC-18 in SQL. Day and month
// boundaries are derived from now (UTC) and passed as bind parameters, so the
// queries stay parameterised. Every aggregate is computed in PostgreSQL and
// returns whole cents.
func (s *Store) DashboardStats(ctx context.Context, now time.Time) (DashboardStats, error) {
	utcNow := now.UTC()
	dayStart := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)
	monthStart := time.Date(utcNow.Year(), utcNow.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	var stats DashboardStats

	if err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM orders WHERE status IN ('angefragt', 'bestätigt', 'in Arbeit')`,
	).Scan(&stats.OpenOrders); err != nil {
		return DashboardStats{}, fmt.Errorf("count open orders: %w", err)
	}

	if err := s.Pool.QueryRow(ctx,
		`SELECT count(DISTINCT order_id) FROM order_status_history
		  WHERE status = 'fertig' AND changed_at >= $1 AND changed_at < $2`,
		dayStart, dayEnd,
	).Scan(&stats.FinishedToday); err != nil {
		return DashboardStats{}, fmt.Errorf("count orders finished today: %w", err)
	}

	if err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(gross_cents), 0) FROM invoices
		  WHERE issued_at >= $1 AND issued_at < $2`,
		monthStart, monthEnd,
	).Scan(&stats.RevenueMonthCents); err != nil {
		return DashboardStats{}, fmt.Errorf("sum revenue this month: %w", err)
	}

	return stats, nil
}
