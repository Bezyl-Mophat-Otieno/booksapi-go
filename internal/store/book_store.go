package store

import (
	"database/sql"
	"errors"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {

	// SQLite only enforces foreign keys when enabled per connection;
	// the _pragma DSN param applies it to every pooled connection.
	// busy_timeout makes a connection wait (up to 5s) for another connection's
	// write transaction to finish instead of failing immediately with SQLITE_BUSY.
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}


type Book struct {
	Id         int64
	Title      string
	Author     string
	Pages      int64
	PriceCents int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
		`INSERT INTO books  (title, author, pages, price_cents, created_at, updated_at)
	VALUES(?,?,?,?,?,?)`, b.Title, b.Author, b.Pages, b.PriceCents, b.CreatedAt, b.UpdatedAt)

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

var BookNotFound = errors.New("book not found")

func (s *BookStore) GetById(Id int64) (Book, error) {
	var b Book
	err := s.db.QueryRow(`
	SELECT id, title, author, pages, price_cents, created_at, updated_at
	FROM books WHERE id=?
	`, Id).Scan(&b.Id, &b.Title, &b.Author, &b.Pages, &b.PriceCents, &b.CreatedAt, &b.UpdatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, BookNotFound
	}
	return b, nil

}

func (s *BookStore) List() ([]Book, error) {
	rows, err := s.db.Query(`SELECT id, title, author, pages, price_cents, created_at, updated_at FROM books`)
	if err != nil {
		log.Println("List ", err)
	}
	defer rows.Close()
	books := []Book{}

	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.Id, &b.Title, &b.Author, &b.Pages, &b.PriceCents, &b.CreatedAt, &b.UpdatedAt); err != nil {
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
		return BookNotFound
	}
	return nil
}

func (s *BookStore) Update(b *Book) error {
	now := time.Now().UTC()
	b.UpdatedAt = now
	result, err := s.db.Exec(
		`UPDATE books SET title=?, author=?, pages=?, price_cents=?, updated_at=? WHERE id=? `,
		b.Title, b.Author, b.Pages, b.PriceCents, b.UpdatedAt, b.Id,
	)
	if err != nil {
		return err
	}
	return checkFound(result)
}

func (s *BookStore) Delete(id int64) error {
	result, err := s.db.Exec(`DELETE FROM books WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return BookNotFound
	}
	if err != nil {
		return err
	}

	return checkFound(result)
}
