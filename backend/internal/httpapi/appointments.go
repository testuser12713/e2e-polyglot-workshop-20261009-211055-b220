package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"workshop/api/internal/store"
)

// desiredDateFormat is the wire format of the appointment's desired date.
const desiredDateFormat = "2006-01-02"

// appointmentRequest is the body of POST /api/appointments.
type appointmentRequest struct {
	Customer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	} `json:"customer"`
	Vehicle struct {
		Plate   string `json:"plate"`
		Make    string `json:"make"`
		Model   string `json:"model"`
		Mileage int    `json:"mileage"`
	} `json:"vehicle"`
	DesiredDate string `json:"desired_date"`
	Problem     string `json:"problem"`
}

// appointmentResponse is the 201 body: the generated order number and its
// initial status.
type appointmentResponse struct {
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
}

// createAppointment handles POST /api/appointments. It validates the request,
// then creates or reuses customer and vehicle and stores the order in status
// "angefragt" together with its first history entry.
func (s *Server) createAppointment(w http.ResponseWriter, r *http.Request) {
	var req appointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body is not valid JSON")
		return
	}

	desiredDate, err := time.Parse(desiredDateFormat, strings.TrimSpace(req.DesiredDate))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "desired_date must be a date in YYYY-MM-DD format")
		return
	}
	if msg := validateAppointment(&req); msg != "" {
		writeError(w, http.StatusBadRequest, "invalid_request", msg)
		return
	}

	result, err := s.store.CreateAppointment(r.Context(), store.AppointmentInput{
		Customer: store.AppointmentCustomer{
			Name:  strings.TrimSpace(req.Customer.Name),
			Email: strings.TrimSpace(req.Customer.Email),
			Phone: strings.TrimSpace(req.Customer.Phone),
		},
		Vehicle: store.AppointmentVehicle{
			Plate:   strings.TrimSpace(req.Vehicle.Plate),
			Make:    strings.TrimSpace(req.Vehicle.Make),
			Model:   strings.TrimSpace(req.Vehicle.Model),
			Mileage: req.Vehicle.Mileage,
		},
		DesiredDate: desiredDate,
		Problem:     strings.TrimSpace(req.Problem),
	})
	if err != nil {
		// AC-35: never log customer data, only the technical failure.
		log.Printf("httpapi: create appointment failed: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create appointment")
		return
	}

	writeJSON(w, http.StatusCreated, appointmentResponse{
		OrderNumber: result.OrderNumber,
		Status:      result.Status,
	})
}

// validateAppointment returns a human-readable message for the first missing or
// invalid field, or the empty string when the request is acceptable.
func validateAppointment(req *appointmentRequest) string {
	if strings.TrimSpace(req.Customer.Name) == "" {
		return "customer.name is required"
	}
	email := strings.TrimSpace(req.Customer.Email)
	if email == "" {
		return "customer.email is required"
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "customer.email is invalid"
	}
	if strings.TrimSpace(req.Vehicle.Plate) == "" {
		return "vehicle.plate is required"
	}
	if req.Vehicle.Mileage < 0 {
		return "vehicle.mileage must not be negative"
	}
	if strings.TrimSpace(req.Problem) == "" {
		return "problem is required"
	}
	return ""
}
