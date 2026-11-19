package main

import (
	"booksapi/internal/handlers"
	"booksapi/internal/store"
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	db, err := store.Open("books.db")
	if err != nil {
		log.Fatal("Opening database", err)
		db.Close()
	}

	if err := store.Migrate(db); err != nil {
		log.Fatal("Migrating database", err)
	}

	bookStore := store.NewBookStore(db)
	bookHandler := handlers.NewBookHandler(bookStore)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	})

	mux.HandleFunc("POST /books", bookHandler.Create)
	mux.HandleFunc("GET /books/{bookId}", bookHandler.Get)
	mux.HandleFunc("DELETE /books/{bookId}", bookHandler.Delete)
	mux.HandleFunc("PUT /books/{bookId}", bookHandler.Update)
	mux.HandleFunc("GET /books", bookHandler.ListBooks)

	stockStore := store.NewStockStore(db)
	stockHandler := handlers.NewStockHandler(stockStore)

	mux.HandleFunc("POST /books/{bookId}/stock", stockHandler.Create)
	mux.HandleFunc("GET /books/{bookId}/stock", stockHandler.Get)
	mux.HandleFunc("PUT /books/{bookId}/stock", stockHandler.Update)
	mux.HandleFunc("DELETE /books/{bookId}/stock", stockHandler.Delete)
	mux.HandleFunc("POST /books/{bookId}/stock/increase", stockHandler.Increase)
	mux.HandleFunc("POST /books/{bookId}/stock/decrease", stockHandler.Decrease)
	mux.HandleFunc("GET /books/{bookId}/stock/check", stockHandler.Check)
	mux.HandleFunc("GET /stocks", stockHandler.List)

	log.Println("Server running on http://localhost:8080")

	log.Fatal(http.ListenAndServe(":8080", mux))

}
