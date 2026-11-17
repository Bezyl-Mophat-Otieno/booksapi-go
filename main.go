package main

import (
	"encoding/json"
	"fmt"
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

	book := Book{Title: "Things fall apart", Author: "Chinua Achebe"}

	err = newBookStore.Create(&book)

	log.Println("Created a book of ID: ", book.Id)

	if err != nil {
		log.Fatal("Failed to save a book", err)
	}

	found, err := newBookStore.GetById(book.Id)
	if err != nil {
		log.Fatal("Get", err)
	}

	log.Printf("Found  %s by %s \n", found.Author, found.Title)

	books, err := newBookStore.List()

	if err != nil {
		log.Fatal("list: ", err)
	}
	fmt.Println("Total books:", len(books))
	for _, b := range books {
		fmt.Printf("  #%d %s by %s\n", b.Id, b.Title, b.Author)
	}

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	})

	mux.HandleFunc("GET /hello/{name}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		name := r.PathValue("name")
		json.NewEncoder(w).Encode(map[string]string{"message": "hello" + name})

	})

	log.Println("Server running on http://localhost:8080")

	log.Fatal(http.ListenAndServe(":8080", mux))

}
