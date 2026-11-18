package main

import (
	"database/sql"
	"errors"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			title      TEXT     NOT NULL,
			author     TEXT     NOT NULL,
			pages      INTEGER  NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`)
	return err
}

type Book struct {
	Id        int64
	Title     string
	Author    string
	Pages     int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BookStore struct {
	db *sql.DB
}

func NewBookStore(db *sql.DB) *BookStore {
	return &BookStore{db: db}
}

func (s *BookStore) Create(b *Book) error {
	now := time.Now().UTC()
	b.CreatedAt = now
	b.UpdatedAt = now
	result, err := s.db.Exec(
		`INSERT INTO books  (title, author, pages, created_at, updated_at) 
	VALUES(?,?,?,?,?)`, b.Title, b.Author, b.Pages, b.CreatedAt, b.UpdatedAt)

	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	b.Id = id
	return err
}

var ErrNotFound = errors.New("book not found")

func (s *BookStore) GetById(Id int64) (Book, error) {
	var b Book

	err := s.db.QueryRow(`
	SELECT title, author, pages, created_at, updated_at
	FROM books WHERE id=? 
	`, Id).Scan(&b.Title, &b.Author, &b.Pages, &b.CreatedAt, &b.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	return b, nil

}

func (s *BookStore) List() ([]Book, error) {
	rows, err := s.db.Query(`SELECT title, author, pages, created_at, updated_at FROM books`)
	if err != nil {
		log.Fatal("List ", err)
	}
	defer rows.Close()
	books := []Book{}

	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.Title, &b.Author, &b.Pages, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err

		}
		books = append(books, b)
	}

	return books, rows.Err()

}

func checkFound(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *BookStore) Update(b *Book) error {
	now := time.Now().UTC()
	b.UpdatedAt = now
	result, err := s.db.Exec(
		`UPDATE books SET title=?, author=?, pages=?, updated_at=? WHERE id=? `,
		b.Title, b.Author, b.Pages, b.UpdatedAt, b.Id,
	)
	if err != nil {
		return err
	}
	return checkFound(result)
}

func (s *BookStore) Delete(id int64) error {
	result, err := s.db.Exec(`DELETE FROM books WHERE id = ?`, id)
	if err != nil {
		return err
	}

	return checkFound(result)
}
