package store

import (
	"context"
	"fmt"
)

// Customer is one row of the customers table.
type Customer struct {
	ID    int64
	Name  string
	Email string
	Phone string
}

// CreateCustomer inserts a new customer and returns the stored row. The query
// is parameterised — no value is ever concatenated into the SQL text.
func (s *Store) CreateCustomer(ctx context.Context, name, email, phone string) (*Customer, error) {
	c := &Customer{}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, email, phone`,
		name, email, phone,
	).Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
	if err != nil {
		return nil, fmt.Errorf("insert customer: %w", err)
	}
	return c, nil
}

// FindCustomerByID returns the customer with the given id. A missing row is
// reported as pgx.ErrNoRows so the caller can answer 404.
func (s *Store) FindCustomerByID(ctx context.Context, id int64) (*Customer, error) {
	c := &Customer{}
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, email, phone FROM customers WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
	if err != nil {
		return nil, fmt.Errorf("select customer by id: %w", err)
	}
	return c, nil
}

// FindCustomerByEmail returns the customer with the given e-mail address. A
// missing row is reported as pgx.ErrNoRows so the caller can tell "unknown
// customer" apart from a real database failure.
func (s *Store) FindCustomerByEmail(ctx context.Context, email string) (*Customer, error) {
	c := &Customer{}
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, email, phone FROM customers WHERE email = $1`,
		email,
	).Scan(&c.ID, &c.Name, &c.Email, &c.Phone)
	if err != nil {
		return nil, fmt.Errorf("select customer by email: %w", err)
	}
	return c, nil
}
