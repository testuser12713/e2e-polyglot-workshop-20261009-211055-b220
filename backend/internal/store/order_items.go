package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrOrderNotFound is returned when no order carries the requested number.
var ErrOrderNotFound = errors.New("store: order not found")

// OrderItem is one part position of an order: a description, a quantity and a
// unit price. Money is always a whole number of cents.
type OrderItem struct {
	Description    string
	Quantity       int
	UnitPriceCents int64
}

// FindOrderIDByNumber resolves the internal id of an order from its order
// number. It returns ErrOrderNotFound when the number is unknown.
func (s *Store) FindOrderIDByNumber(ctx context.Context, orderNumber string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `SELECT id FROM orders WHERE order_number = $1`, orderNumber).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrOrderNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("find order by number: %w", err)
	}
	return id, nil
}

// ReplaceOrderPositions atomically replaces the labor minutes and every part
// position of an order: the stored parts are deleted and the given ones
// inserted in a single transaction, so a failure leaves the previous state
// untouched. All values are bound parameters. It returns ErrOrderNotFound when
// the order vanished between the lookup and the update.
func (s *Store) ReplaceOrderPositions(ctx context.Context, orderID int64, laborMinutes int, items []OrderItem) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin positions transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE orders SET labor_minutes = $1, updated_at = now() WHERE id = $2`,
		laborMinutes, orderID)
	if err != nil {
		return fmt.Errorf("update order labor minutes: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOrderNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, orderID); err != nil {
		return fmt.Errorf("delete order items: %w", err)
	}

	for _, item := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO order_items (order_id, description, quantity, unit_price_cents)
			 VALUES ($1, $2, $3, $4)`,
			orderID, item.Description, item.Quantity, item.UnitPriceCents); err != nil {
			return fmt.Errorf("insert order item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit positions transaction: %w", err)
	}
	return nil
}

// GetOrderPositions returns the stored labor minutes and part positions of an
// order. It returns ErrOrderNotFound when the order does not exist.
func (s *Store) GetOrderPositions(ctx context.Context, orderID int64) (int, []OrderItem, error) {
	var laborMinutes int
	err := s.Pool.QueryRow(ctx, `SELECT labor_minutes FROM orders WHERE id = $1`, orderID).Scan(&laborMinutes)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, ErrOrderNotFound
	}
	if err != nil {
		return 0, nil, fmt.Errorf("read order labor minutes: %w", err)
	}

	rows, err := s.Pool.Query(ctx,
		`SELECT description, quantity, unit_price_cents
		 FROM order_items WHERE order_id = $1 ORDER BY id`, orderID)
	if err != nil {
		return 0, nil, fmt.Errorf("read order items: %w", err)
	}
	defer rows.Close()

	items := []OrderItem{}
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.Description, &item.Quantity, &item.UnitPriceCents); err != nil {
			return 0, nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("iterate order items: %w", err)
	}
	return laborMinutes, items, nil
}
