package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"workshop/api/internal/store"
)

// maxPositionsBodyBytes caps the request body so a huge payload cannot exhaust
// memory. The positions of a single order are far smaller than this.
const maxPositionsBodyBytes = 1 << 20

// positionPart is one part of an order: a description, a quantity and a unit
// price in whole cents.
type positionPart struct {
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

// positionsRequest is the body of PUT /api/workshop/orders/{number}/positions.
type positionsRequest struct {
	LaborMinutes int            `json:"labor_minutes"`
	Parts        []positionPart `json:"parts"`
}

// positionsResponse is the saved state of an order's positions.
type positionsResponse struct {
	LaborMinutes int            `json:"labor_minutes"`
	Parts        []positionPart `json:"parts"`
}

// updatePositions handles PUT /api/workshop/orders/{number}/positions. It
// validates the submitted labor minutes and parts, then replaces the stored
// positions of the order in one transaction and answers 200 with the saved
// values. Invalid values are rejected with 400, an unknown order with 404.
func (s *Server) updatePositions(w http.ResponseWriter, r *http.Request) {
	number := pathParam(r, "number")

	var req positionsRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPositionsBodyBytes))
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Anfrage konnte nicht gelesen werden")
		return
	}

	if message, ok := validatePositions(req); !ok {
		writeError(w, http.StatusBadRequest, "validation_error", message)
		return
	}

	orderID, err := s.store.FindOrderIDByNumber(r.Context(), number)
	if errors.Is(err, store.ErrOrderNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Auftrag nicht gefunden")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Interner Fehler")
		return
	}

	parts := make([]store.OrderItem, 0, len(req.Parts))
	for _, part := range req.Parts {
		parts = append(parts, store.OrderItem{
			Description:    strings.TrimSpace(part.Description),
			Quantity:       part.Quantity,
			UnitPriceCents: part.UnitPriceCents,
		})
	}

	if err := s.store.ReplaceOrderPositions(r.Context(), orderID, req.LaborMinutes, parts); err != nil {
		if errors.Is(err, store.ErrOrderNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Auftrag nicht gefunden")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Interner Fehler")
		return
	}

	response := positionsResponse{LaborMinutes: req.LaborMinutes, Parts: make([]positionPart, 0, len(req.Parts))}
	for _, part := range parts {
		response.Parts = append(response.Parts, positionPart{
			Description:    part.Description,
			Quantity:       part.Quantity,
			UnitPriceCents: part.UnitPriceCents,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

// validatePositions checks the submitted positions and returns a German
// message and false on the first invalid value. Negative labor minutes, a
// quantity below 1, a negative price and a missing description are all
// rejected (AC-04).
func validatePositions(req positionsRequest) (string, bool) {
	if req.LaborMinutes < 0 {
		return "Arbeitszeit darf nicht negativ sein", false
	}
	for _, part := range req.Parts {
		if strings.TrimSpace(part.Description) == "" {
			return "Teilebeschreibung darf nicht leer sein", false
		}
		if part.Quantity < 1 {
			return "Menge muss mindestens 1 sein", false
		}
		if part.UnitPriceCents < 0 {
			return "Einzelpreis darf nicht negativ sein", false
		}
	}
	return "", true
}
