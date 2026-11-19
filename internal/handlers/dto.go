package handlers

import (
	"booksapi/internal/store"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

type CreateBookRequest struct {
	Title      string `json:"title"`
	Author     string `json:"author"`
	Pages      int64  `json:"pages"`
	PriceCents int64  `json:"price_cents"`
}

type UpdateBookRequest struct {
	Title      *string `json:"title"`
	Author     *string `json:"author"`
	Pages      *int64  `json:"pages"`
	PriceCents *int64  `json:"price_cents"`
}

func (u UpdateBookRequest) Validate() error {
	if u.Title != nil && strings.TrimSpace(*u.Title) == "" {
		return errors.New("Title is required")
	}
	if u.Author != nil && strings.TrimSpace(*u.Author) == "" {
		return errors.New("Author is required")
	}

	if u.Pages != nil && *u.Pages < 0 {
		return errors.New("Pages can not be negative")
	}
	if u.PriceCents != nil && *u.PriceCents < 0 {
		return errors.New("Price cannot be negative")
	}
	return nil
}

func (r CreateBookRequest) Validate() error {
	if strings.TrimSpace(r.Title) == "" {
		return errors.New("Title is required")
	}
	if strings.TrimSpace(r.Author) == "" {
		return errors.New("Author is required")
	}
	if r.Pages < 0 {
		return errors.New("Pages cannot be negative")
	}
	if r.PriceCents < 0 {
		return errors.New("Price cannot be negative")
	}
	return nil
}

type BookResponse struct {
	Id         int64  `json:"id"`
	Title      string `json:"title"`
	Author     string `json:"author"`
	Pages      int    `json:"pages"`
	PriceCents int64  `json:"price_cents"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func NewBookResponse(b store.Book) BookResponse {
	return BookResponse{
		Id:         b.Id,
		Title:      b.Title,
		Author:     b.Author,
		Pages:      int(b.Pages),
		PriceCents: b.PriceCents,
		CreatedAt:  b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  b.UpdatedAt.Format(time.RFC3339),
	}
}

func ToBooksResponses(books []store.Book) []BookResponse {
	responses := make([]BookResponse, 0, len(books))
	for _, b := range books {
		responses = append(responses, NewBookResponse(b))
	}
	return responses
}

// Stock

type CreateStockRequest struct {
	Quantity int64 `json:"quantity"`
}

func (r CreateStockRequest) Validate() error {
	if r.Quantity < 0 {
		return errors.New("Quantity cannot be negative")
	}
	return nil
}

type UpdateStockRequest struct {
	Quantity *int64 `json:"quantity"`
}

func (r UpdateStockRequest) Validate() error {
	if r.Quantity == nil {
		return errors.New("Quantity is required")
	}
	if *r.Quantity < 0 {
		return errors.New("Quantity cannot be negative")
	}
	return nil
}

// AdjustStockRequest is used for both increasing and decreasing stock.
type AdjustStockRequest struct {
	Amount int64 `json:"amount"`
}

func (r AdjustStockRequest) Validate() error {
	if r.Amount <= 0 {
		return errors.New("Amount must be greater than zero")
	}
	return nil
}

type StockResponse struct {
	Id        int64  `json:"id"`
	BookId    int64  `json:"book_id"`
	Quantity  int64  `json:"quantity"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func NewStockResponse(s store.Stock) StockResponse {
	return StockResponse{
		Id:        s.Id,
		BookId:    s.BookId,
		Quantity:  s.Quantity,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
		UpdatedAt: s.UpdatedAt.Format(time.RFC3339),
	}
}

func ToStockResponses(stocks []store.Stock) []StockResponse {
	responses := make([]StockResponse, 0, len(stocks))
	for _, s := range stocks {
		responses = append(responses, NewStockResponse(s))
	}
	return responses
}

type StockCheckResponse struct {
	BookId    int64 `json:"book_id"`
	Requested int64 `json:"requested"`
	Available int64 `json:"available"`
	InStock   bool  `json:"in_stock"`
}

// Orders

const maxOrderItems = 100

type OrderItemRequest struct {
	BookId   int64 `json:"book_id"`
	Quantity int64 `json:"quantity"`
}

type CreateOrderRequest struct {
	CustomerName  string             `json:"customer_name"`
	CustomerEmail string             `json:"customer_email"`
	CustomerPhone string             `json:"customer_phone"`
	Notes         string             `json:"notes"`
	Items         []OrderItemRequest `json:"items"`
}

// phonePattern: optional leading +, then 7-15 digits (E.164 length), checked
// after spaces, dashes and brackets have been removed by normalizePhone.
var phonePattern = regexp.MustCompile(`^\+?[0-9]{7,15}$`)

func normalizePhone(phone string) string {
	return strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(phone))
}

func (r CreateOrderRequest) Validate() error {
	if strings.TrimSpace(r.CustomerName) == "" {
		return errors.New("Customer name is required")
	}

	email := strings.TrimSpace(r.CustomerEmail)
	if email == "" {
		return errors.New("Customer email is required")
	}
	// ParseAddress also accepts "Name <a@b.com>", so require the bare address.
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return errors.New("Customer email is not a valid email address")
	}

	if strings.TrimSpace(r.CustomerPhone) == "" {
		return errors.New("Customer phone is required")
	}
	if !phonePattern.MatchString(normalizePhone(r.CustomerPhone)) {
		return errors.New("Customer phone must be 7 to 15 digits, optionally starting with +")
	}

	if len(r.Items) == 0 {
		return errors.New("An order must have at least one item")
	}
	if len(r.Items) > maxOrderItems {
		return fmt.Errorf("An order cannot have more than %d items", maxOrderItems)
	}
	seen := make(map[int64]bool, len(r.Items))
	for _, item := range r.Items {
		if item.BookId <= 0 {
			return errors.New("Each item needs a valid book_id")
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("Quantity for book %d must be greater than zero", item.BookId)
		}
		if seen[item.BookId] {
			return fmt.Errorf("Book %d appears more than once; combine it into one item", item.BookId)
		}
		seen[item.BookId] = true
	}
	return nil
}

// ToOrder converts a validated request into a store.Order, trimming the
// customer details and normalizing the phone number.
func (r CreateOrderRequest) ToOrder() store.Order {
	items := make([]store.OrderItem, 0, len(r.Items))
	for _, item := range r.Items {
		items = append(items, store.OrderItem{BookId: item.BookId, Quantity: item.Quantity})
	}
	return store.Order{
		CustomerName:  strings.TrimSpace(r.CustomerName),
		CustomerEmail: strings.TrimSpace(r.CustomerEmail),
		CustomerPhone: normalizePhone(r.CustomerPhone),
		Notes:         strings.TrimSpace(r.Notes),
		Items:         items,
	}
}

type UpdateOrderStatusRequest struct {
	Status store.OrderStatus `json:"status"`
}

func (r UpdateOrderStatusRequest) Validate() error {
	if r.Status == "" {
		return errors.New("Status is required")
	}
	if !r.Status.Valid() {
		return fmt.Errorf("Unknown status %q; use placed, paid, dispatched or cancelled", r.Status)
	}
	return nil
}

type OrderItemResponse struct {
	Id             int64 `json:"id"`
	BookId         int64 `json:"book_id"`
	Quantity       int64 `json:"quantity"`
	UnitPriceCents int64 `json:"unit_price_cents"`
	LineTotalCents int64 `json:"line_total_cents"`
}

type OrderResponse struct {
	Id            int64               `json:"id"`
	CustomerName  string              `json:"customer_name"`
	CustomerEmail string              `json:"customer_email"`
	CustomerPhone string              `json:"customer_phone"`
	Notes         string              `json:"notes"`
	Status        store.OrderStatus   `json:"status"`
	TotalCents    int64               `json:"total_cents"`
	PlacedAt      string              `json:"placed_at"`
	PaidAt        *string             `json:"paid_at"`
	DispatchedAt  *string             `json:"dispatched_at"`
	CancelledAt   *string             `json:"cancelled_at"`
	UpdatedAt     string              `json:"updated_at"`
	Items         []OrderItemResponse `json:"items,omitempty"` // omitted in list responses
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func NewOrderResponse(o store.Order) OrderResponse {
	var items []OrderItemResponse
	if o.Items != nil {
		items = make([]OrderItemResponse, 0, len(o.Items))
		for _, it := range o.Items {
			items = append(items, OrderItemResponse{
				Id:             it.Id,
				BookId:         it.BookId,
				Quantity:       it.Quantity,
				UnitPriceCents: it.UnitPriceCents,
				LineTotalCents: it.LineTotalCents(),
			})
		}
	}
	return OrderResponse{
		Id:            o.Id,
		CustomerName:  o.CustomerName,
		CustomerEmail: o.CustomerEmail,
		CustomerPhone: o.CustomerPhone,
		Notes:         o.Notes,
		Status:        o.Status,
		TotalCents:    o.TotalCents,
		PlacedAt:      o.PlacedAt.Format(time.RFC3339),
		PaidAt:        formatOptionalTime(o.PaidAt),
		DispatchedAt:  formatOptionalTime(o.DispatchedAt),
		CancelledAt:   formatOptionalTime(o.CancelledAt),
		UpdatedAt:     o.UpdatedAt.Format(time.RFC3339),
		Items:         items,
	}
}

func ToOrderResponses(orders []store.Order) []OrderResponse {
	responses := make([]OrderResponse, 0, len(orders))
	for _, o := range orders {
		responses = append(responses, NewOrderResponse(o))
	}
	return responses
}

type StockShortageResponse struct {
	BookId    int64 `json:"book_id"`
	Requested int64 `json:"requested"`
	Available int64 `json:"available"`
}
