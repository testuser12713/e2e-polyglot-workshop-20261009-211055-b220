package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrEmployeeNotFound is returned by the employee lookups when no row matches.
var ErrEmployeeNotFound = errors.New("employee not found")

// Employee is a workshop account. The password exists only as the bcrypt hash
// in PasswordHash; the type is never marshalled to JSON, so no response can
// leak it (AC-30).
type Employee struct {
	ID           int
	Email        string
	Name         string
	PasswordHash string
}

// FindEmployeeByEmail returns the employee with the given e-mail address, or
// ErrEmployeeNotFound when the account does not exist.
func (s *Store) FindEmployeeByEmail(ctx context.Context, email string) (*Employee, error) {
	var e Employee
	err := s.Pool.QueryRow(ctx,
		`SELECT id, email, name, password_hash FROM employees WHERE email = $1`,
		email,
	).Scan(&e.ID, &e.Email, &e.Name, &e.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEmployeeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find employee by email: %w", err)
	}
	return &e, nil
}

// CreateEmployee inserts a new employee row and returns its id. passwordHash
// must already be a password hash (bcrypt), never a clear-text password.
func (s *Store) CreateEmployee(ctx context.Context, email, name, passwordHash string) (int, error) {
	var id int
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO employees (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		email, name, passwordHash,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create employee: %w", err)
	}
	return id, nil
}

// EmployeesExist reports whether the employees table holds at least one row.
// The first-employee seed uses it to run exactly once.
func (s *Store) EmployeesExist(ctx context.Context) (bool, error) {
	var exists bool
	if err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM employees)`,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check employees: %w", err)
	}
	return exists, nil
}
