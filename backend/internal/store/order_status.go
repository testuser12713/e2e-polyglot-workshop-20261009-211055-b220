package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// OrderStatusFlow is the strictly ordered status path of a workshop order
// (AC-05). A status change is only legal from one entry to the very next one.
var OrderStatusFlow = []string{"angefragt", "bestätigt", "in Arbeit", "fertig", "abgeholt"}

// Sentinel errors returned by AdvanceOrderStatus so the HTTP layer can map them
// to 404 and 409 without parsing strings.
var (
	// ErrOrderStatusNotFound is returned when no order carries the number.
	ErrOrderStatusNotFound = errors.New("order not found")
	// ErrInvalidStatusTransition is returned for a repeat, a jump or an
	// unknown status — anything that is not the single next step.
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)

// OrderStatusChange is one row of an order's status history.
type OrderStatusChange struct {
	Status    string    `json:"status"`
	ChangedAt time.Time `json:"changed_at"`
}

// OrderStatusAdvanceResult is what a committed status change returns to the
// caller: the order id (needed for the invoice queue message), the reached
// status and the full history in chronological order.
type OrderStatusAdvanceResult struct {
	OrderID int64
	Status  string
	History []OrderStatusChange
}

// AdvanceOrderStatus moves an order to target if and only if target is the
// single next step of OrderStatusFlow. The move is one transaction: the row is
// locked, the transition validated, orders.status updated and an
// order_status_history row stamped with the current UTC time inserted. The
// history is read back before the commit so the returned snapshot matches the
// committed state.
func (s *Store) AdvanceOrderStatus(ctx context.Context, orderNumber, target string) (*OrderStatusAdvanceResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin status transaction: %w", err)
	}
	// Rollback is a no-op once the transaction is committed.
	defer func() { _ = tx.Rollback(ctx) }()

	var orderID int64
	var current string
	err = tx.QueryRow(ctx,
		`SELECT id, status FROM orders WHERE order_number = $1 FOR UPDATE`, orderNumber).
		Scan(&orderID, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderStatusNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load order %q: %w", orderNumber, err)
	}

	next, ok := nextOrderStatus(current)
	if !ok || next != target {
		return nil, ErrInvalidStatusTransition
	}

	if _, err := tx.Exec(ctx,
		`UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`, target, orderID); err != nil {
		return nil, fmt.Errorf("update order %d status: %w", orderID, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status, changed_at) VALUES ($1, $2, now())`,
		orderID, target); err != nil {
		return nil, fmt.Errorf("insert order %d status history: %w", orderID, err)
	}

	history, err := readOrderStatusHistory(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit status transaction: %w", err)
	}

	return &OrderStatusAdvanceResult{OrderID: orderID, Status: target, History: history}, nil
}

// readOrderStatusHistory returns the history rows of one order in chronological
// order. Every timestamp is normalised to UTC so the JSON body always carries a
// UTC instant (AC-06).
func readOrderStatusHistory(ctx context.Context, tx pgx.Tx, orderID int64) ([]OrderStatusChange, error) {
	rows, err := tx.Query(ctx,
		`SELECT status, changed_at FROM order_status_history
		 WHERE order_id = $1 ORDER BY changed_at, id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("read order %d status history: %w", orderID, err)
	}
	defer rows.Close()

	history := make([]OrderStatusChange, 0)
	for rows.Next() {
		var change OrderStatusChange
		if err := rows.Scan(&change.Status, &change.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan order %d status history: %w", orderID, err)
		}
		change.ChangedAt = change.ChangedAt.UTC()
		history = append(history, change)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order %d status history: %w", orderID, err)
	}
	return history, nil
}

// nextOrderStatus returns the single legal successor of current, or ok=false
// when current is the last status (or is unknown).
func nextOrderStatus(current string) (string, bool) {
	for i, status := range OrderStatusFlow {
		if status != current {
			continue
		}
		if i+1 < len(OrderStatusFlow) {
			return OrderStatusFlow[i+1], true
		}
		return "", false
	}
	return "", false
}
