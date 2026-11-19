package handlers

import (
	"booksapi/internal/store"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

type OrderHandler struct {
	store *store.OrderStore
}

func NewOrderHandler(store *store.OrderStore) *OrderHandler {
	return &OrderHandler{store: store}
}

func parseOrderId(w http.ResponseWriter, r *http.Request) (int64, bool) {
	orderId, err := strconv.ParseInt(r.PathValue("orderId"), 10, 64)
	if err != nil || orderId <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid OrderId")
		return 0, false
	}
	return orderId, true
}

func writeOrderError(w http.ResponseWriter, err error, action string) {
	var shortageErr *store.InsufficientStockError
	var transitionErr *store.InvalidTransitionError
	var bookErr *store.BookNotFoundError

	switch {
	case errors.As(err, &shortageErr):
		shortages := make([]StockShortageResponse, 0, len(shortageErr.Shortages))
		for _, s := range shortageErr.Shortages {
			shortages = append(shortages, StockShortageResponse{
				BookId: s.BookId, Requested: s.Requested, Available: s.Available,
			})
		}
		writeJson(w, http.StatusConflict, map[string]any{
			"error":     "Insufficient stock",
			"shortages": shortages,
		})
	case errors.As(err, &transitionErr):
		allowed := transitionErr.From.AllowedNext()
		if allowed == nil {
			allowed = []store.OrderStatus{}
		}
		writeJson(w, http.StatusConflict, map[string]any{
			"error":          transitionErr.Error(),
			"current_status": transitionErr.From,
			"allowed_next":   allowed,
		})
	case errors.As(err, &bookErr):
		writeError(w, http.StatusNotFound, bookErr.Error())
	case errors.Is(err, store.ErrOrderNotFound):
		writeError(w, http.StatusNotFound, "Order Not Found")
	case errors.Is(err, store.ErrOrderStatusChanged):
		writeError(w, http.StatusConflict, "Order status was changed by another request; fetch the order and retry")
	default:
		log.Println(action, err)
		writeError(w, http.StatusInternalServerError, "Could not "+action)
	}
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	order := req.ToOrder()
	if err := h.store.Create(&order); err != nil {
		writeOrderError(w, err, "create order")
		return
	}

	writeJson(w, http.StatusCreated, NewOrderResponse(order))
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	orderId, ok := parseOrderId(w, r)
	if !ok {
		return
	}

	order, err := h.store.GetById(orderId)
	if err != nil {
		writeOrderError(w, err, "fetch order")
		return
	}

	writeJson(w, http.StatusOK, NewOrderResponse(order))
}

// List returns orders newest first. ?status=paid filters by status.
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	status := store.OrderStatus(r.URL.Query().Get("status"))
	if status != "" && !status.Valid() {
		writeError(w, http.StatusBadRequest, "Unknown status; use placed, paid, dispatched or cancelled")
		return
	}

	orders, err := h.store.List(status)
	if err != nil {
		writeOrderError(w, err, "fetch orders")
		return
	}

	writeJson(w, http.StatusOK, ToOrderResponses(orders))
}

func (h *OrderHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	orderId, ok := parseOrderId(w, r)
	if !ok {
		return
	}

	var req UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	order, err := h.store.UpdateStatus(orderId, req.Status)
	if err != nil {
		writeOrderError(w, err, "update order status")
		return
	}

	writeJson(w, http.StatusOK, NewOrderResponse(order))
}
