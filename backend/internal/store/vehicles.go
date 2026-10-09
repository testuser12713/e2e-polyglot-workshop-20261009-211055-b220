package store

import (
	"context"
	"fmt"
)

// Vehicle is one row of the vehicles table. Plate is unique across the table.
type Vehicle struct {
	ID      int64
	Plate   string
	Make    string
	Model   string
	Mileage int
}

// CreateVehicle inserts a new vehicle and returns the stored row. A duplicate
// plate makes PostgreSQL raise a unique-violation error, which the handler maps
// to 409. The query is parameterised.
func (s *Store) CreateVehicle(ctx context.Context, plate, make, model string, mileage int) (*Vehicle, error) {
	v := &Vehicle{}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO vehicles (plate, make, model, mileage)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, plate, make, model, mileage`,
		plate, make, model, mileage,
	).Scan(&v.ID, &v.Plate, &v.Make, &v.Model, &v.Mileage)
	if err != nil {
		return nil, fmt.Errorf("insert vehicle: %w", err)
	}
	return v, nil
}

// FindVehicleByPlate returns the vehicle with the given plate. A missing row is
// reported as pgx.ErrNoRows so the caller can answer 404.
func (s *Store) FindVehicleByPlate(ctx context.Context, plate string) (*Vehicle, error) {
	v := &Vehicle{}
	err := s.Pool.QueryRow(ctx,
		`SELECT id, plate, make, model, mileage FROM vehicles WHERE plate = $1`,
		plate,
	).Scan(&v.ID, &v.Plate, &v.Make, &v.Model, &v.Mileage)
	if err != nil {
		return nil, fmt.Errorf("select vehicle by plate: %w", err)
	}
	return v, nil
}
