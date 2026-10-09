package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// customerResponse is the JSON representation of a customer. It never carries
// anything but the fields of AC-01.
type customerResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// customerCreateRequest is the body of POST /api/customers.
type customerCreateRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// createCustomer handles POST /api/customers. It answers 201 with the created
// customer, or 400 in the uniform error body when the name is missing or the
// e-mail is invalid (AC-01).
func (s *Server) createCustomer(w http.ResponseWriter, r *http.Request) {
	var req customerCreateRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	phone := strings.TrimSpace(req.Phone)

	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_name", "name is required")
		return
	}
	if !validCustomerEmail(email) {
		writeError(w, http.StatusBadRequest, "invalid_email", "a valid e-mail address is required")
		return
	}

	customer, err := s.store.CreateCustomer(r.Context(), name, email, phone)
	if err != nil {
		// AC-35: never log the e-mail address or any other customer value.
		log.Printf("httpapi: create customer: database error")
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create customer")
		return
	}

	writeJSON(w, http.StatusCreated, customerResponse{
		ID:    customer.ID,
		Name:  customer.Name,
		Email: customer.Email,
		Phone: customer.Phone,
	})
}

// getCustomer handles GET /api/customers/{id}. An unknown or non-numeric id
// answers 404 in the uniform error body (AC-01).
func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(pathParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "not_found", "customer not found")
		return
	}

	customer, err := s.store.FindCustomerByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "customer not found")
		return
	}
	if err != nil {
		log.Printf("httpapi: get customer: database error")
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load customer")
		return
	}

	writeJSON(w, http.StatusOK, customerResponse{
		ID:    customer.ID,
		Name:  customer.Name,
		Email: customer.Email,
		Phone: customer.Phone,
	})
}

// validCustomerEmail reports whether s is a plausible e-mail address: exactly
// one "@", a non-empty local part, and a domain with at least one dot. It
// deliberately does not log or echo the address.
func validCustomerEmail(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	domain := s[at+1:]
	return strings.Contains(domain, ".") &&
		!strings.HasPrefix(domain, ".") &&
		!strings.HasSuffix(domain, ".")
}
