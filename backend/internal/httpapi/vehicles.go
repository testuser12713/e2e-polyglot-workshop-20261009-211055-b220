package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// vehicleResponse is the JSON representation of a vehicle (AC-02).
type vehicleResponse struct {
	ID      int64  `json:"id"`
	Plate   string `json:"plate"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	Mileage int    `json:"mileage"`
}

// vehicleCreateRequest is the body of POST /api/vehicles.
type vehicleCreateRequest struct {
	Plate   string `json:"plate"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	Mileage int    `json:"mileage"`
}

// createVehicle handles POST /api/vehicles. It answers 201 with the created
// vehicle, 400 for invalid values (missing plate or negative mileage), and 409
// when the plate already exists (AC-02).
func (s *Server) createVehicle(w http.ResponseWriter, r *http.Request) {
	var req vehicleCreateRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	plate := strings.ToUpper(strings.TrimSpace(req.Plate))
	make := strings.TrimSpace(req.Make)
	model := strings.TrimSpace(req.Model)

	if plate == "" {
		writeError(w, http.StatusBadRequest, "invalid_plate", "plate is required")
		return
	}
	if req.Mileage < 0 {
		writeError(w, http.StatusBadRequest, "invalid_mileage", "mileage must not be negative")
		return
	}

	vehicle, err := s.store.CreateVehicle(r.Context(), plate, make, model, req.Mileage)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// The duplicate-key message echoes the plate, so it must never be
			// logged (AC-35).
			writeError(w, http.StatusConflict, "plate_taken", "a vehicle with this plate already exists")
			return
		}
		log.Printf("httpapi: create vehicle: database error")
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create vehicle")
		return
	}

	writeJSON(w, http.StatusCreated, vehicleResponse{
		ID:      vehicle.ID,
		Plate:   vehicle.Plate,
		Make:    vehicle.Make,
		Model:   vehicle.Model,
		Mileage: vehicle.Mileage,
	})
}
