package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// StatusRequested is the status an appointment request starts in. It is the
// first state of the strictly forward-only workshop workflow.
const StatusRequested = "angefragt"

// AppointmentCustomer is the customer half of an appointment request. A
// customer is reused when one with the same e-mail already exists.
type AppointmentCustomer struct {
	Name  string
	Email string
	Phone string
}

// AppointmentVehicle is the vehicle half of an appointment request. A vehicle
// is reused when one with the same plate already exists.
type AppointmentVehicle struct {
	Plate   string
	Make    string
	Model   string
	Mileage int
}

// AppointmentInput carries everything needed to create one requested order.
type AppointmentInput struct {
	Customer    AppointmentCustomer
	Vehicle     AppointmentVehicle
	DesiredDate time.Time
	Problem     string
}

// AppointmentResult is what the API answers after a successful insert.
type AppointmentResult struct {
	OrderNumber string
	Status      string
}

// CreateAppointment creates or reuses the customer (by e-mail) and the vehicle
// (by plate), then inserts the order in status "angefragt" with its first
// order_status_history row and a consecutive AU-YYYY-NNNN number. All of it
// runs in ONE transaction, so a failure halfway leaves nothing behind.
func (s *Store) CreateAppointment(ctx context.Context, in AppointmentInput) (AppointmentResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return AppointmentResult{}, fmt.Errorf("begin appointment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	customerID, err := upsertCustomer(ctx, tx, in.Customer)
	if err != nil {
		return AppointmentResult{}, err
	}
	vehicleID, err := upsertVehicle(ctx, tx, in.Vehicle, customerID)
	if err != nil {
		return AppointmentResult{}, err
	}
	orderNumber, err := nextOrderNumber(ctx, tx, time.Now().UTC().Year())
	if err != nil {
		return AppointmentResult{}, err
	}

	var orderID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		orderNumber, customerID, vehicleID, StatusRequested, in.DesiredDate, in.Problem,
	).Scan(&orderID); err != nil {
		return AppointmentResult{}, fmt.Errorf("insert order: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status) VALUES ($1, $2)`,
		orderID, StatusRequested,
	); err != nil {
		return AppointmentResult{}, fmt.Errorf("insert order status history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AppointmentResult{}, fmt.Errorf("commit appointment transaction: %w", err)
	}
	return AppointmentResult{OrderNumber: orderNumber, Status: StatusRequested}, nil
}

// upsertCustomer returns the id of the customer with this e-mail, inserting a
// new row when none exists yet.
func upsertCustomer(ctx context.Context, tx pgx.Tx, c AppointmentCustomer) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx,
		`SELECT id FROM customers WHERE email = $1 ORDER BY id LIMIT 1`,
		c.Email,
	).Scan(&id)
	switch {
	case err == nil:
		return id, nil
	case errors.Is(err, pgx.ErrNoRows):
		// No existing customer — insert one below.
	default:
		return 0, fmt.Errorf("look up customer by e-mail: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		c.Name, c.Email, c.Phone,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("insert customer: %w", err)
	}
	return id, nil
}

// upsertVehicle returns the id of the vehicle with this plate, inserting a new
// row when none exists yet and refreshing the known one otherwise.
func upsertVehicle(ctx context.Context, tx pgx.Tx, v AppointmentVehicle, customerID int64) (int64, error) {
	var id int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage, customer_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (plate) DO UPDATE SET
		     make        = EXCLUDED.make,
		     model       = EXCLUDED.model,
		     mileage     = EXCLUDED.mileage,
		     customer_id = EXCLUDED.customer_id
		 RETURNING id`,
		v.Plate, v.Make, v.Model, v.Mileage, customerID,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("upsert vehicle: %w", err)
	}
	return id, nil
}

// nextOrderNumber atomically increments the order counter and formats it as
// AU-YYYY-NNNN, the customer-facing order number. The UPDATE on the counter row
// serialises concurrent requests, so two appointments never receive the same
// number.
func nextOrderNumber(ctx context.Context, tx pgx.Tx, year int) (string, error) {
	var seq int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO counters (name, value) VALUES ('aw', 1)
		 ON CONFLICT (name) DO UPDATE SET value = counters.value + 1
		 RETURNING value`,
	).Scan(&seq); err != nil {
		return "", fmt.Errorf("increment order counter: %w", err)
	}
	return fmt.Sprintf("AU-%04d-%04d", year, seq), nil
}
