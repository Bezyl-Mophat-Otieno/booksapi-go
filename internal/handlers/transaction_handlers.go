package handlers

import (
	"booksapi/internal/store"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

type TransactionHandler struct {
	store *store.TransactionStore
}

func NewTransactionHandler(store *store.TransactionStore) *TransactionHandler {
	return &TransactionHandler{store: store}
}

func parseTransactionId(w http.ResponseWriter, r *http.Request) (int64, bool) {
	transactionId, err := strconv.ParseInt(r.PathValue("transactionId"), 10, 64)
	if err != nil || transactionId <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid TransactionId")
		return 0, false
	}
	return transactionId, true
}

func writeTransactionError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, store.ErrOrderNotFound):
		writeError(w, http.StatusNotFound, "Order Not Found")
	case errors.Is(err, store.ErrTransactionNotFound):
		writeError(w, http.StatusNotFound, "Transaction Not Found")
	default:
		log.Println(action, err)
		writeError(w, http.StatusInternalServerError, "Could not "+action)
	}
}

func (h *TransactionHandler) Create(w http.ResponseWriter, r *http.Request) {
	orderId, ok := parseOrderId(w, r)
	if !ok {
		return
	}

	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	transaction := req.ToTransaction(orderId)
	if err := h.store.Create(&transaction); err != nil {
		writeTransactionError(w, err, "create transaction")
		return
	}

	writeJson(w, http.StatusCreated, NewTransactionResponse(transaction))
}

func (h *TransactionHandler) Get(w http.ResponseWriter, r *http.Request) {
	transactionId, ok := parseTransactionId(w, r)
	if !ok {
		return
	}

	transaction, err := h.store.GetById(transactionId)
	if err != nil {
		writeTransactionError(w, err, "fetch transaction")
		return
	}

	writeJson(w, http.StatusOK, NewTransactionResponse(transaction))
}

func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	transactions, err := h.store.List(r.URL.Query().Get("order_tracking_id"))
	if err != nil {
		writeTransactionError(w, err, "fetch transactions")
		return
	}

	writeJson(w, http.StatusOK, ToTransactionResponses(transactions))
}

func (h *TransactionHandler) ListByOrder(w http.ResponseWriter, r *http.Request) {
	orderId, ok := parseOrderId(w, r)
	if !ok {
		return
	}

	transactions, err := h.store.ListByOrder(orderId)
	if err != nil {
		writeTransactionError(w, err, "fetch transactions")
		return
	}

	writeJson(w, http.StatusOK, ToTransactionResponses(transactions))
}
