package handlers

import (
	"errors"
	"strings"
	"time"
	"booksapi/internal/store"
)

type CreateBookRequest struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Pages  int64  `json:"pages"`
}

type UpdateBookRequest struct {
	Title  *string `json:"title"`
	Author *string `json:"author"`
	Pages  *int64  `json:"pages"`
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
	return nil
}

type BookResponse struct {
	Id        int64  `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Pages     int    `json:"pages"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func NewBookResponse(b store.Book) BookResponse {
	return BookResponse{
		Id:        b.Id,
		Title:     b.Title,
		Author:    b.Author,
		Pages:     int(b.Pages),
		CreatedAt: b.CreatedAt.Format(time.RFC3339),
		UpdatedAt: b.UpdatedAt.Format(time.RFC3339),
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
