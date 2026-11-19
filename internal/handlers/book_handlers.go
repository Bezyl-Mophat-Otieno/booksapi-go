package handlers

import (
	"booksapi/internal/store"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
)

type BookHandler struct {
	store *store.BookStore
}

func NewBookHandler(store *store.BookStore) *BookHandler {
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

func parseBookId(w http.ResponseWriter, r *http.Request) (int64, bool) {
	bookId, err := strconv.ParseInt(r.PathValue("bookId"), 10, 64)
	if err != nil || bookId <= 0 {
		writeError(w, http.StatusBadRequest, "Invalid BookId")
		return 0, false
	}
	return bookId, true
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

	book := store.Book{Title: req.Title, Author: req.Author, Pages: req.Pages}

	if err := h.store.Create(&book); err != nil {
		log.Println("Save book", err)
		writeError(w, http.StatusInternalServerError, "Could not save book")
		return
	}

	writeJson(w, http.StatusCreated, NewBookResponse(book))

}
func (h *BookHandler) Get(w http.ResponseWriter, r *http.Request) {
	bookId, ok := parseBookId(w, r)
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid BookId")
		return
	}

	found, err := h.store.GetById(bookId)

	if errors.Is(err, store.BookNotFound) {
		writeError(w, http.StatusNotFound, "Book Not Found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not fetch book")
		return
	}
	writeJson(w, http.StatusFound, NewBookResponse(found))
}
func (h *BookHandler) ListBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.List()
	if err != nil {
		log.Println("List Books", err)
		writeError(w, http.StatusInternalServerError, "Could not fetch books")
		return
	}

	writeJson(w, http.StatusOK, ToBooksResponses(books))

}
func (h *BookHandler) Update(w http.ResponseWriter, r *http.Request) {

	var req UpdateBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Json Body")
		return
	}

	bookId, ok := parseBookId(w, r)
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid BookId")
		return
	}

	found, err := h.store.GetById(bookId)

	if errors.Is(err, store.BookNotFound) {
		writeError(w, http.StatusNotFound, "Book Not Found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not fetch book")
		return
	}

	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Title != nil {
		found.Title = *req.Title
	}
	if req.Author != nil {
		found.Author = *req.Author
	}
	if req.Pages != nil {
		found.Pages = *req.Pages
	}

	if err := h.store.Update(&found); err != nil {
		log.Println("Update book", err)
		writeError(w, http.StatusBadRequest, "Could not Update book")
		return
	}

	writeJson(w, http.StatusOK, NewBookResponse(found))

}
func (h *BookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	bookId, ok := parseBookId(w, r)
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid BookId")
		return
	}

	err := h.store.Delete(bookId)

	if errors.Is(err, store.BookNotFound) {
		writeError(w, http.StatusNotFound, "Book Not Found")
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not delete book")
		return
	}
	writeJson(w, http.StatusOK, map[string]string{"message": "Book Deleted successfully"})

}
