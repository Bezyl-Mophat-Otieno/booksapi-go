package main

import (
	"database/sql"
	"time"
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

func (s *BookStore) Create(b *Book) error{
	now:= time.Now().UTC()
	b.CreatedAt = now
	b.UpdatedAt = now
	result, err := s.db.Exec(
	`INSERT INTO (Title, Author, Pages, CreatedAt, UpdatedAt) 
	VALUES(?,?,?,?,?)`, b.Title, b.Author, b.Pages, b.CreatedAt, b.UpdatedAt )
	
	if err != nil {
		return  err
	}
	id, err := result.LastInsertId()
	if err !=nil {
		return err
	}
	b.Id = id
	return err
}