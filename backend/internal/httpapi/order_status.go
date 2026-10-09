package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"workshop/api/internal/store"
)

// invoiceQueueName is the Valkey list the invoice worker consumes (AC-07). The
// message for a finished order is exactly {"order_id": <id>}.
const invoiceQueueName = "invoices"

// updateOrderStatus handles POST /api/workshop/orders/{number}/status. It
// accepts only the single next step of the ordered status path; a repeat, a
// jump or an unknown value is answered with 409 in the uniform error body, an
// unknown order number with 404 (AC-05).
func (s *Server) updateOrderStatus(w http.ResponseWriter, r *http.Request) {
	number := pathParam(r, "number")

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body must be JSON with a status field")
		return
	}

	result, err := s.store.AdvanceOrderStatus(r.Context(), number, payload.Status)
	switch {
	case errors.Is(err, store.ErrOrderStatusNotFound):
		writeError(w, http.StatusNotFound, "not_found", "order not found")
		return
	case errors.Is(err, store.ErrInvalidStatusTransition):
		writeError(w, http.StatusConflict, "invalid_transition",
			"status may only advance one step along angefragt, bestätigt, in Arbeit, fertig, abgeholt")
		return
	case err != nil:
		log.Printf("httpapi: advance status of %q failed: %v", number, err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	// The message is enqueued only after the transaction committed, so a rolled
	// back status change never leaves an invoice job behind (AC-07).
	if result.Status == "fertig" {
		message, err := json.Marshal(map[string]int64{"order_id": result.OrderID})
		if err != nil {
			log.Printf("httpapi: encode invoice message for order %d: %v", result.OrderID, err)
		} else if err := s.queue.Push(r.Context(), invoiceQueueName, string(message)); err != nil {
			log.Printf("httpapi: push invoice message for order %d: %v", result.OrderID, err)
		}
	}

	writeJSON(w, http.StatusOK, struct {
		Status  string                    `json:"status"`
		History []store.OrderStatusChange `json:"history"`
	}{Status: result.Status, History: result.History})
}
