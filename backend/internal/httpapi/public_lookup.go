package httpapi

import (
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"workshop/api/internal/store"
)

// lookupVehicleDTO is the vehicle part of the public status response.
type lookupVehicleDTO struct {
	Plate   string `json:"plate"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	Mileage int    `json:"mileage"`
}

// lookupHistoryDTO is one status history entry of the public status response.
type lookupHistoryDTO struct {
	Status    string `json:"status"`
	ChangedAt string `json:"changed_at"`
}

// lookupStatusResponse is the body of GET /api/orders/{number}/status.
type lookupStatusResponse struct {
	OrderNumber string             `json:"order_number"`
	Status      string             `json:"status"`
	Vehicle     lookupVehicleDTO   `json:"vehicle"`
	Problem     string             `json:"problem"`
	History     []lookupHistoryDTO `json:"history"`
}

// lookupItemDTO is one invoice line of the public invoice response.
type lookupItemDTO struct {
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

// lookupInvoiceResponse is the body of GET /api/orders/{number}/invoice. Every
// monetary value is a whole number of cents.
type lookupInvoiceResponse struct {
	InvoiceNumber string          `json:"invoice_number"`
	IssuedAt      string          `json:"issued_at"`
	LaborMinutes  int             `json:"labor_minutes"`
	Items         []lookupItemDTO `json:"items"`
	NetCents      int64           `json:"net_cents"`
	VatCents      int64           `json:"vat_cents"`
	GrossCents    int64           `json:"gross_cents"`
}

// lookupOrderStatus handles GET /api/orders/{number}/status. It is public (no
// token) and applies the shared public-lookup limiter (AC-36).
func (s *Server) lookupOrderStatus(w http.ResponseWriter, r *http.Request) {
	if !s.allowPublicLookup(r) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
		return
	}

	number := pathParam(r, "number")
	plate := r.URL.Query().Get("plate")

	result, err := s.store.LookupOrderStatus(r.Context(), number, plate)
	if errors.Is(err, store.ErrOrderLookupNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no order matches the given number and plate")
		return
	}
	if err != nil {
		log.Printf("httpapi: lookup order status failed: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	response := lookupStatusResponse{
		OrderNumber: result.OrderNumber,
		Status:      result.Status,
		Vehicle: lookupVehicleDTO{
			Plate:   result.Vehicle.Plate,
			Make:    result.Vehicle.Make,
			Model:   result.Vehicle.Model,
			Mileage: result.Vehicle.Mileage,
		},
		Problem: result.Problem,
		History: make([]lookupHistoryDTO, 0, len(result.History)),
	}
	for _, entry := range result.History {
		response.History = append(response.History, lookupHistoryDTO{
			Status:    entry.Status,
			ChangedAt: entry.ChangedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, response)
}

// lookupOrderInvoice handles GET /api/orders/{number}/invoice. It is public (no
// token) and applies the shared public-lookup limiter (AC-36).
func (s *Server) lookupOrderInvoice(w http.ResponseWriter, r *http.Request) {
	if !s.allowPublicLookup(r) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
		return
	}

	number := pathParam(r, "number")
	plate := r.URL.Query().Get("plate")

	result, err := s.store.LookupOrderInvoice(r.Context(), number, plate)
	if errors.Is(err, store.ErrOrderLookupNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "no invoice matches the given number and plate")
		return
	}
	if err != nil {
		log.Printf("httpapi: lookup order invoice failed: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	response := lookupInvoiceResponse{
		InvoiceNumber: result.InvoiceNumber,
		IssuedAt:      result.IssuedAt.UTC().Format(time.RFC3339),
		LaborMinutes:  result.LaborMinutes,
		Items:         make([]lookupItemDTO, 0, len(result.Items)),
		NetCents:      result.NetCents,
		VatCents:      result.VatCents,
		GrossCents:    result.GrossCents,
	}
	for _, item := range result.Items {
		response.Items = append(response.Items, lookupItemDTO{
			Description:    item.Description,
			Quantity:       item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

// allowPublicLookup records a hit for the calling client IP and reports whether
// it is still inside the per-minute budget. Status and invoice share the one
// limiter, so the two public endpoints together observe the 20/minute cap.
func (s *Server) allowPublicLookup(r *http.Request) bool {
	return s.lookupLimiter.Allow(publicLookupClient(r))
}

// publicLookupClient extracts the client IP from the request, falling back to
// the raw RemoteAddr when it does not carry a port. It never reads forwarded
// headers, so a client cannot mint a fresh budget by spoofing one.
func publicLookupClient(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return r.RemoteAddr
	}
	return host
}
