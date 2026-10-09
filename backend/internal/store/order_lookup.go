package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrOrderLookupNotFound is returned when the public lookup does not find an
// order matching both the order number and the plate, or when no invoice
// exists for that order yet. It maps to 404 at the HTTP layer.
var ErrOrderLookupNotFound = errors.New("order lookup not found")

// OrderLookupVehicle is the vehicle data exposed by the public status lookup.
type OrderLookupVehicle struct {
	Plate   string
	Make    string
	Model   string
	Mileage int
}

// OrderLookupHistory is one entry of an order's status history.
type OrderLookupHistory struct {
	Status    string
	ChangedAt time.Time
}

// OrderLookupStatus is everything the public status lookup returns.
type OrderLookupStatus struct {
	OrderNumber string
	Status      string
	Vehicle     OrderLookupVehicle
	Problem     string
	History     []OrderLookupHistory
}

// OrderLookupItem is one invoice line of the public invoice lookup.
type OrderLookupItem struct {
	Description    string
	Quantity       int
	UnitPriceCents int64
}

// OrderLookupInvoice is everything the public invoice lookup returns. Every
// monetary value is a whole number of cents.
type OrderLookupInvoice struct {
	InvoiceNumber string
	IssuedAt      time.Time
	LaborMinutes  int
	Items         []OrderLookupItem
	NetCents      int64
	VatCents      int64
	GrossCents    int64
}

// LookupOrderStatus returns the status, vehicle data, problem description and
// status history of the order matching number AND plate. A mismatching plate
// and an unknown order number are deliberately indistinguishable: both yield
// ErrOrderLookupNotFound (AC-12). Read-only, parameterised SQL.
func (s *Store) LookupOrderStatus(ctx context.Context, number, plate string) (*OrderLookupStatus, error) {
	var out OrderLookupStatus
	err := s.Pool.QueryRow(ctx, `
		SELECT o.order_number, o.status, o.problem,
		       v.plate, v.make, v.model, v.mileage
		FROM orders o
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.order_number = $1 AND v.plate = $2`,
		number, plate,
	).Scan(
		&out.OrderNumber, &out.Status, &out.Problem,
		&out.Vehicle.Plate, &out.Vehicle.Make, &out.Vehicle.Model, &out.Vehicle.Mileage,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderLookupNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup order status: %w", err)
	}

	out.History = make([]OrderLookupHistory, 0)
	rows, err := s.Pool.Query(ctx, `
		SELECT h.status, h.changed_at
		FROM order_status_history h
		JOIN orders o ON o.id = h.order_id
		WHERE o.order_number = $1
		ORDER BY h.changed_at, h.id`,
		number,
	)
	if err != nil {
		return nil, fmt.Errorf("lookup order history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry OrderLookupHistory
		if err := rows.Scan(&entry.Status, &entry.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan order history: %w", err)
		}
		out.History = append(out.History, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read order history: %w", err)
	}

	return &out, nil
}

// LookupOrderInvoice returns the invoice of the order matching number AND
// plate. A missing invoice and a mismatching plate both yield
// ErrOrderLookupNotFound (AC-13). Read-only, parameterised SQL.
func (s *Store) LookupOrderInvoice(ctx context.Context, number, plate string) (*OrderLookupInvoice, error) {
	var out OrderLookupInvoice
	err := s.Pool.QueryRow(ctx, `
		SELECT i.invoice_number, i.issued_at, i.labor_minutes,
		       i.net_cents, i.vat_cents, i.gross_cents
		FROM invoices i
		JOIN orders o ON o.id = i.order_id
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.order_number = $1 AND v.plate = $2`,
		number, plate,
	).Scan(
		&out.InvoiceNumber, &out.IssuedAt, &out.LaborMinutes,
		&out.NetCents, &out.VatCents, &out.GrossCents,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderLookupNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup order invoice: %w", err)
	}

	out.Items = make([]OrderLookupItem, 0)
	rows, err := s.Pool.Query(ctx, `
		SELECT oi.description, oi.quantity, oi.unit_price_cents
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.order_number = $1
		ORDER BY oi.id`,
		number,
	)
	if err != nil {
		return nil, fmt.Errorf("lookup invoice items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item OrderLookupItem
		if err := rows.Scan(&item.Description, &item.Quantity, &item.UnitPriceCents); err != nil {
			return nil, fmt.Errorf("scan invoice item: %w", err)
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read invoice items: %w", err)
	}

	return &out, nil
}
