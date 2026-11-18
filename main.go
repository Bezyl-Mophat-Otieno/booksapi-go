package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	db, err := openDB("books.db")
	if err != nil {
		log.Fatal("Opening database", err)
		db.Close()
	}

	if err := migrate(db); err != nil {
		log.Fatal("Migrating database", err)
	}

	newBookStore := NewBookStore(db)
	books := NewBookHandler(newBookStore)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	})

	mux.HandleFunc("POST /books", books.Create)
	mux.HandleFunc("GET /books/{bookId}", books.Get)
	mux.HandleFunc("DELETE /books/{bookId}", books.Delete)
	mux.HandleFunc("PUT /books/{bookId}", books.Get)
	mux.HandleFunc("GET /books", books.ListBooks)

	log.Println("Server running on http://localhost:8080")

	log.Fatal(http.ListenAndServe(":8080", mux))

}
