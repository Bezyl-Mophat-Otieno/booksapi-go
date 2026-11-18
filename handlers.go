package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type BookHandler struct {
	store *BookStore
}

func NewBookHandler(store *BookStore) *BookHandler {
	return &BookHandler{store: store}
}

func writeJson(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJson(w, status, map[string]string{"error": message})
}

func (h *BookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	book := Book{Title: req.Title, Author: req.Author, Pages: req.Pages}

	if err := h.store.Create(&book); err != nil {
		log.Println("Save book", err)
		writeError(w, http.StatusInternalServerError, "Could not save book")
		return
	}

	writeJson(w, http.StatusCreated, NewBookResponse(book))

}
