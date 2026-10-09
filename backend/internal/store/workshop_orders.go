package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrWorkshopOrderNotFound is returned by GetWorkshopOrder when no order
// carries the requested number. The HTTP layer turns it into a uniform 404.
var ErrWorkshopOrderNotFound = errors.New("workshop order not found")

// WorkshopOrderSummary is one row of the workshop order list.
type WorkshopOrderSummary struct {
	OrderNumber  string
	Status       string
	Plate        string
	CustomerName string
}

// WorkshopOrderHeader is the order part of the workshop detail.
type WorkshopOrderHeader struct {
	ID           int64
	OrderNumber  string
	Status       string
	DesiredDate  *time.Time
	Problem      string
	LaborMinutes int64
	CustomerID   *int64
	VehicleID    *int64
}

// WorkshopCustomer is the customer part of the workshop detail. Its fields
// deliberately carry no password or hash.
type WorkshopCustomer struct {
	ID    int64
	Name  string
	Email string
	Phone string
}

// WorkshopVehicle is the vehicle part of the workshop detail.
type WorkshopVehicle struct {
	Plate   string
	Make    string
	Model   string
	Mileage int
}

// WorkshopOrderPart is one part position of an order.
type WorkshopOrderPart struct {
	Description    string
	Quantity       int
	UnitPriceCents int64
}

// WorkshopHistoryEntry is one logged status change.
type WorkshopHistoryEntry struct {
	Status    string
	ChangedAt time.Time
}

// WorkshopInvoice is the invoice part of the workshop detail.
type WorkshopInvoice struct {
	InvoiceNumber string
	IssuedAt      time.Time
	LaborMinutes  int64
	NetCents      int64
	VatCents      int64
	GrossCents    int64
}

// WorkshopOrderDetail is everything the workshop detail endpoint returns.
type WorkshopOrderDetail struct {
	Order    WorkshopOrderHeader
	Customer *WorkshopCustomer
	Vehicle  *WorkshopVehicle
	Parts    []WorkshopOrderPart
	History  []WorkshopHistoryEntry
	Invoice  *WorkshopInvoice
}

// ListWorkshopOrders returns the order list, optionally filtered by status and
// searched by plate. Empty parameters mean "no filter"; every filter is a bind
// parameter, so no SQL text is ever assembled from user input.
func (s *Store) ListWorkshopOrders(ctx context.Context, status, plate string) ([]WorkshopOrderSummary, error) {
	const query = `
		SELECT o.order_number,
		       o.status,
		       COALESCE(v.plate, ''),
		       COALESCE(c.name, '')
		FROM orders o
		LEFT JOIN vehicles v ON v.id = o.vehicle_id
		LEFT JOIN customers c ON c.id = o.customer_id
		WHERE ($1 = '' OR o.status = $1)
		  AND ($2 = '' OR v.plate ILIKE '%' || $2 || '%')
		ORDER BY o.created_at DESC, o.id DESC`

	rows, err := s.Pool.Query(ctx, query, status, plate)
	if err != nil {
		return nil, fmt.Errorf("list workshop orders: %w", err)
	}
	defer rows.Close()

	summaries := make([]WorkshopOrderSummary, 0)
	for rows.Next() {
		var item WorkshopOrderSummary
		if err := rows.Scan(&item.OrderNumber, &item.Status, &item.Plate, &item.CustomerName); err != nil {
			return nil, fmt.Errorf("scan workshop order summary: %w", err)
		}
		summaries = append(summaries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workshop orders: %w", err)
	}
	return summaries, nil
}

// GetWorkshopOrder returns one order with its customer, vehicle, positions,
// history and (when present) invoice. It returns ErrWorkshopOrderNotFound when
// the order number is unknown.
func (s *Store) GetWorkshopOrder(ctx context.Context, number string) (*WorkshopOrderDetail, error) {
	const orderQuery = `
		SELECT id, order_number, status, desired_date, problem, labor_minutes,
		       customer_id, vehicle_id
		FROM orders
		WHERE order_number = $1`

	detail := &WorkshopOrderDetail{}
	err := s.Pool.QueryRow(ctx, orderQuery, number).Scan(
		&detail.Order.ID,
		&detail.Order.OrderNumber,
		&detail.Order.Status,
		&detail.Order.DesiredDate,
		&detail.Order.Problem,
		&detail.Order.LaborMinutes,
		&detail.Order.CustomerID,
		&detail.Order.VehicleID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrWorkshopOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load workshop order: %w", err)
	}

	if detail.Order.CustomerID != nil {
		customer := &WorkshopCustomer{}
		err := s.Pool.QueryRow(ctx,
			`SELECT id, name, email, phone FROM customers WHERE id = $1`,
			*detail.Order.CustomerID,
		).Scan(&customer.ID, &customer.Name, &customer.Email, &customer.Phone)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("load workshop order customer: %w", err)
		}
		if err == nil {
			detail.Customer = customer
		}
	}

	if detail.Order.VehicleID != nil {
		vehicle := &WorkshopVehicle{}
		err := s.Pool.QueryRow(ctx,
			`SELECT plate, make, model, mileage FROM vehicles WHERE id = $1`,
			*detail.Order.VehicleID,
		).Scan(&vehicle.Plate, &vehicle.Make, &vehicle.Model, &vehicle.Mileage)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("load workshop order vehicle: %w", err)
		}
		if err == nil {
			detail.Vehicle = vehicle
		}
	}

	parts, err := s.listWorkshopOrderParts(ctx, detail.Order.ID)
	if err != nil {
		return nil, err
	}
	detail.Parts = parts

	history, err := s.listWorkshopOrderHistory(ctx, detail.Order.ID)
	if err != nil {
		return nil, err
	}
	detail.History = history

	invoice, err := s.getWorkshopOrderInvoice(ctx, detail.Order.ID)
	if err != nil {
		return nil, err
	}
	detail.Invoice = invoice

	return detail, nil
}

func (s *Store) listWorkshopOrderParts(ctx context.Context, orderID int64) ([]WorkshopOrderPart, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT description, quantity, unit_price_cents
		 FROM order_items
		 WHERE order_id = $1
		 ORDER BY id`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("list workshop order parts: %w", err)
	}
	defer rows.Close()

	parts := make([]WorkshopOrderPart, 0)
	for rows.Next() {
		var part WorkshopOrderPart
		if err := rows.Scan(&part.Description, &part.Quantity, &part.UnitPriceCents); err != nil {
			return nil, fmt.Errorf("scan workshop order part: %w", err)
		}
		parts = append(parts, part)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workshop order parts: %w", err)
	}
	return parts, nil
}

func (s *Store) listWorkshopOrderHistory(ctx context.Context, orderID int64) ([]WorkshopHistoryEntry, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT status, changed_at
		 FROM order_status_history
		 WHERE order_id = $1
		 ORDER BY changed_at, id`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("list workshop order history: %w", err)
	}
	defer rows.Close()

	history := make([]WorkshopHistoryEntry, 0)
	for rows.Next() {
		var entry WorkshopHistoryEntry
		if err := rows.Scan(&entry.Status, &entry.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan workshop order history: %w", err)
		}
		history = append(history, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workshop order history: %w", err)
	}
	return history, nil
}

func (s *Store) getWorkshopOrderInvoice(ctx context.Context, orderID int64) (*WorkshopInvoice, error) {
	invoice := &WorkshopInvoice{}
	err := s.Pool.QueryRow(ctx,
		`SELECT invoice_number, issued_at, labor_minutes, net_cents, vat_cents, gross_cents
		 FROM invoices
		 WHERE order_id = $1`,
		orderID,
	).Scan(
		&invoice.InvoiceNumber,
		&invoice.IssuedAt,
		&invoice.LaborMinutes,
		&invoice.NetCents,
		&invoice.VatCents,
		&invoice.GrossCents,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load workshop order invoice: %w", err)
	}
	return invoice, nil
}
