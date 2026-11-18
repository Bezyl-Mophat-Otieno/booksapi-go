package main

import (
	"errors"
	"strings"
	"time"
)

type CreateBookRequest struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Pages  int64  `json:"pages"`
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

func NewBookResponse(b Book) BookResponse {
	return BookResponse{
		Id:        b.Id,
		Title:     b.Title,
		Author:    b.Author,
		Pages:     int(b.Pages),
		CreatedAt: b.CreatedAt.Format(time.RFC3339),
		UpdatedAt: b.UpdatedAt.Format(time.RFC3339),
	}
}
