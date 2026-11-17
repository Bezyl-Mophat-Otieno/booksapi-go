package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// A struct is a named group of fields
type Book struct {
	Id        int    `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Published bool   `json:"published"`
}

type BookRequest struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

func (b BookRequest) BookRequestValidator() error {
	if strings.TrimSpace(b.Author) == "" {
		return errors.New("Author is a required field")
	} else if strings.TrimSpace(b.Title) == "" {
		return errors.New("Author is a required field")
	} else {
		return nil
	}
}

func (b BookRequest) ToBook() Book {
	id := time.Now().Nanosecond()
	return Book{
		Id:        id,
		Title:     b.Title,
		Author:    b.Author,
		Published: false,
	}
}

type BookResponse struct {
	Id        int    `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Published bool   `json:"published"`
}

func (b Book) Summary() string {
	return "Book: " + b.Title + "by " + b.Author
}

func (b *Book) Publish() {
	b.Published = true
}

func main() {

	book1 := Book{Id: 1, Title: "The Great Gatsby", Author: "F. Scott Fitzgerald"}

	fmt.Printf("My name is %v, the author of %v \n", book1.Author, book1.Title)

	book2 := book1

	fmt.Printf("My name is %v, the author of %v\n", book2.Author, book2.Title)

	book3 := &book1

	book3.Author = "John Doe"
	book3.Title = "The art of war"

	fmt.Printf("My name is %v, the author of %v\n", book3.Author, book3.Title)
	fmt.Printf("My name is %v, the author of %v \n", book1.Author, book1.Title)

	// Receivers. This is the way we attach behaviour to a struct

	fmt.Println(book1.Summary())
	fmt.Println(book1.Author + "" + book1.Title + "")
	book1.Publish()
	fmt.Println(book1.Published)

	bookRequest := BookRequest{Title: "My new Book", Author: "Kwame Nkrumah"}
	bookFromBookRequest := bookRequest.ToBook()

	fmt.Println("This is a book from a book request: " + bookFromBookRequest.Author + " " + bookFromBookRequest.Title)
	fmt.Println(bookFromBookRequest.Summary())
}
