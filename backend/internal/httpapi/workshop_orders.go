package httpapi

import (
	"errors"
	"log"
	"net/http"
	"time"

	"workshop/api/internal/store"
)

// workshopOrderSummaryBody is one order row of the list response.
type workshopOrderSummaryBody struct {
	OrderNumber  string `json:"order_number"`
	Status       string `json:"status"`
	Plate        string `json:"plate"`
	CustomerName string `json:"customer_name"`
}

// workshopOrderListBody is the body of GET /api/workshop/orders.
type workshopOrderListBody struct {
	Orders []workshopOrderSummaryBody `json:"orders"`
}

// workshopOrderBody is the order part of the detail response.
type workshopOrderBody struct {
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
	DesiredDate string `json:"desired_date"`
	Problem     string `json:"problem"`
}

// workshopCustomerBody is the customer part of the detail response.
type workshopCustomerBody struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// workshopVehicleBody is the vehicle part of the detail response.
type workshopVehicleBody struct {
	Plate   string `json:"plate"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	Mileage int    `json:"mileage"`
}

// workshopPositionBody is one part position.
type workshopPositionBody struct {
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

// workshopPositionsBody bundles the labor minutes with the parts.
type workshopPositionsBody struct {
	LaborMinutes int64                  `json:"labor_minutes"`
	Parts        []workshopPositionBody `json:"parts"`
}

// workshopHistoryBody is one status change.
type workshopHistoryBody struct {
	Status    string    `json:"status"`
	ChangedAt time.Time `json:"changed_at"`
}

// workshopInvoiceBody is the invoice part of the detail response.
type workshopInvoiceBody struct {
	InvoiceNumber string                 `json:"invoice_number"`
	IssuedAt      time.Time              `json:"issued_at"`
	LaborMinutes  int64                  `json:"labor_minutes"`
	Items         []workshopPositionBody `json:"items"`
	NetCents      int64                  `json:"net_cents"`
	VatCents      int64                  `json:"vat_cents"`
	GrossCents    int64                  `json:"gross_cents"`
}

// workshopOrderDetailBody is the body of GET /api/workshop/orders/{number}.
type workshopOrderDetailBody struct {
	Order     workshopOrderBody     `json:"order"`
	Customer  workshopCustomerBody  `json:"customer"`
	Vehicle   workshopVehicleBody   `json:"vehicle"`
	Positions workshopPositionsBody `json:"positions"`
	History   []workshopHistoryBody `json:"history"`
	Invoice   *workshopInvoiceBody  `json:"invoice"`
}

// listWorkshopOrders handles GET /api/workshop/orders. Empty status and plate
// query parameters mean "no filter".
func (s *Server) listWorkshopOrders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	status := query.Get("status")
	plate := query.Get("plate")

	orders, err := s.store.ListWorkshopOrders(r.Context(), status, plate)
	if err != nil {
		log.Printf("httpapi: list workshop orders failed: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load orders")
		return
	}

	body := workshopOrderListBody{Orders: make([]workshopOrderSummaryBody, 0, len(orders))}
	for _, order := range orders {
		body.Orders = append(body.Orders, workshopOrderSummaryBody{
			OrderNumber:  order.OrderNumber,
			Status:       order.Status,
			Plate:        order.Plate,
			CustomerName: order.CustomerName,
		})
	}
	writeJSON(w, http.StatusOK, body)
}

// getWorkshopOrder handles GET /api/workshop/orders/{number}. It answers 404 in
// the uniform error body for an unknown order number.
func (s *Server) getWorkshopOrder(w http.ResponseWriter, r *http.Request) {
	number := pathParam(r, "number")

	detail, err := s.store.GetWorkshopOrder(r.Context(), number)
	if errors.Is(err, store.ErrWorkshopOrderNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "order not found")
		return
	}
	if err != nil {
		log.Printf("httpapi: get workshop order failed: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load order")
		return
	}

	writeJSON(w, http.StatusOK, toWorkshopOrderDetailBody(detail))
}

func toWorkshopOrderDetailBody(detail *store.WorkshopOrderDetail) workshopOrderDetailBody {
	body := workshopOrderDetailBody{
		Order: workshopOrderBody{
			OrderNumber: detail.Order.OrderNumber,
			Status:      detail.Order.Status,
			Problem:     detail.Order.Problem,
		},
		Customer: workshopCustomerBody{},
		Vehicle:  workshopVehicleBody{},
		Positions: workshopPositionsBody{
			LaborMinutes: detail.Order.LaborMinutes,
			Parts:        toWorkshopPositionBodies(detail.Parts),
		},
		History: make([]workshopHistoryBody, 0, len(detail.History)),
		Invoice: nil,
	}
	if detail.Order.DesiredDate != nil {
		body.Order.DesiredDate = detail.Order.DesiredDate.Format("2006-01-02")
	}
	if detail.Customer != nil {
		body.Customer = workshopCustomerBody{
			ID:    detail.Customer.ID,
			Name:  detail.Customer.Name,
			Email: detail.Customer.Email,
			Phone: detail.Customer.Phone,
		}
	}
	if detail.Vehicle != nil {
		body.Vehicle = workshopVehicleBody{
			Plate:   detail.Vehicle.Plate,
			Make:    detail.Vehicle.Make,
			Model:   detail.Vehicle.Model,
			Mileage: detail.Vehicle.Mileage,
		}
	}
	for _, entry := range detail.History {
		body.History = append(body.History, workshopHistoryBody{
			Status:    entry.Status,
			ChangedAt: entry.ChangedAt,
		})
	}
	if detail.Invoice != nil {
		body.Invoice = &workshopInvoiceBody{
			InvoiceNumber: detail.Invoice.InvoiceNumber,
			IssuedAt:      detail.Invoice.IssuedAt,
			LaborMinutes:  detail.Invoice.LaborMinutes,
			Items:         toWorkshopPositionBodies(detail.Parts),
			NetCents:      detail.Invoice.NetCents,
			VatCents:      detail.Invoice.VatCents,
			GrossCents:    detail.Invoice.GrossCents,
		}
	}
	return body
}

func toWorkshopPositionBodies(parts []store.WorkshopOrderPart) []workshopPositionBody {
	bodies := make([]workshopPositionBody, 0, len(parts))
	for _, part := range parts {
		bodies = append(bodies, workshopPositionBody{
			Description:    part.Description,
			Quantity:       part.Quantity,
			UnitPriceCents: part.UnitPriceCents,
		})
	}
	return bodies
}
