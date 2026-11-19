package handlers


import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"booksapi/internal/store"
)
type StockHandler struct {
	store *store.StockStore
}

func NewStockHandler(store *store.StockStore) *StockHandler {
	return &StockHandler{store: store}
}


func writeStockError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, store.BookNotFound):
		writeError(w, http.StatusNotFound, "Book Not Found")
	case errors.Is(err, store.ErrStockNotFound):
		writeError(w, http.StatusNotFound, "Stock Not Found")
	case errors.Is(err, store.ErrStockExists):
		writeError(w, http.StatusConflict, "Stock already exists for this book")
	case errors.Is(err, store.ErrInsufficientStock):
		writeError(w, http.StatusConflict, "Insufficient stock")
	default:
		log.Println(action, err)
		writeError(w, http.StatusInternalServerError, "Could not "+action)
	}
}

func (h *StockHandler) Create(w http.ResponseWriter, r *http.Request) {
	bookId, ok := parseBookId(w, r)
	if !ok {
		return
	}

	var req CreateStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	stock := store.Stock{BookId: bookId, Quantity: req.Quantity}
	if err := h.store.Create(&stock); err != nil {
		writeStockError(w, err, "create stock")
		return
	}

	writeJson(w, http.StatusCreated, NewStockResponse(stock))
}

func (h *StockHandler) Get(w http.ResponseWriter, r *http.Request) {
	bookId, ok := parseBookId(w, r)
	if !ok {
		return
	}

	stock, err := h.store.GetByBookId(bookId)
	if err != nil {
		writeStockError(w, err, "fetch stock")
		return
	}

	writeJson(w, http.StatusOK, NewStockResponse(stock))
}

func (h *StockHandler) List(w http.ResponseWriter, r *http.Request) {
	stocks, err := h.store.List()
	if err != nil {
		writeStockError(w, err, "fetch stocks")
		return
	}

	writeJson(w, http.StatusOK, ToStockResponses(stocks))
}

func (h *StockHandler) Update(w http.ResponseWriter, r *http.Request) {
	bookId, ok := parseBookId(w, r)
	if !ok {
		return
	}

	var req UpdateStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	stock, err := h.store.SetQuantity(bookId, *req.Quantity)
	if err != nil {
		writeStockError(w, err, "update stock")
		return
	}

	writeJson(w, http.StatusOK, NewStockResponse(stock))
}

func (h *StockHandler) Increase(w http.ResponseWriter, r *http.Request) {
	h.adjust(w, r, h.store.Increase, "increase stock")
}

func (h *StockHandler) Decrease(w http.ResponseWriter, r *http.Request) {
	h.adjust(w, r, h.store.Decrease, "decrease stock")
}

func (h *StockHandler) adjust(w http.ResponseWriter, r *http.Request, op func(bookId, amount int64) (store.Stock, error), action string) {
	bookId, ok := parseBookId(w, r)
	if !ok {
		return
	}

	var req AdjustStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	stock, err := op(bookId, req.Amount)
	if err != nil {
		writeStockError(w, err, action)
		return
	}

	writeJson(w, http.StatusOK, NewStockResponse(stock))
}

// Check reports whether ?quantity=N units of the book are available.
func (h *StockHandler) Check(w http.ResponseWriter, r *http.Request) {
	bookId, ok := parseBookId(w, r)
	if !ok {
		return
	}

	quantity, err := strconv.ParseInt(r.URL.Query().Get("quantity"), 10, 64)
	if err != nil || quantity <= 0 {
		writeError(w, http.StatusBadRequest, "quantity query parameter must be a positive integer")
		return
	}

	inStock, stock, err := h.store.HasQuantity(bookId, quantity)
	if err != nil {
		writeStockError(w, err, "check stock")
		return
	}

	writeJson(w, http.StatusOK, StockCheckResponse{
		BookId:    bookId,
		Requested: quantity,
		Available: stock.Quantity,
		InStock:   inStock,
	})
}

func (h *StockHandler) Delete(w http.ResponseWriter, r *http.Request) {
	bookId, ok := parseBookId(w, r)
	if !ok {
		return
	}

	if err := h.store.Delete(bookId); err != nil {
		writeStockError(w, err, "delete stock")
		return
	}

	writeJson(w, http.StatusOK, map[string]string{"message": "Stock deleted successfully"})
}
