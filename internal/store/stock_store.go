package store

import (
	"database/sql"
	"errors"
	"time"
)

// Stock - The number of books in stock

type Stock struct {
	Id        int64
	BookId    int64
	Quantity  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	ErrStockNotFound     = errors.New("stock not found")
	ErrStockExists       = errors.New("stock already exists for this book")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type StockStore struct {
	db *sql.DB
}

func NewStockStore(db *sql.DB) *StockStore {
	return &StockStore{db: db}
}

const stockColumns = `id, book_id, quantity, created_at, updated_at`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanStock(row rowScanner) (Stock, error) {
	var st Stock
	err := row.Scan(&st.Id, &st.BookId, &st.Quantity, &st.CreatedAt, &st.UpdatedAt)
	return st, err
}

// Create adds a stock record for an existing book.
// Returns BookNotFound if the book does not exist and ErrStockExists if it already has stock.
func (s *StockStore) Create(st *Stock) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var bookExists, stockExists bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM books WHERE id = ?)`, st.BookId).Scan(&bookExists); err != nil {
		return err
	}
	if !bookExists {
		return BookNotFound
	}
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM stocks WHERE book_id = ?)`, st.BookId).Scan(&stockExists); err != nil {
		return err
	}
	if stockExists {
		return ErrStockExists
	}

	now := time.Now().UTC()
	st.CreatedAt = now
	st.UpdatedAt = now
	result, err := tx.Exec(
		`INSERT INTO stocks (book_id, quantity, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		st.BookId, st.Quantity, st.CreatedAt, st.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	st.Id = id

	return tx.Commit()
}

func (s *StockStore) GetByBookId(bookId int64) (Stock, error) {
	st, err := scanStock(s.db.QueryRow(`SELECT `+stockColumns+` FROM stocks WHERE book_id = ?`, bookId))
	if errors.Is(err, sql.ErrNoRows) {
		return Stock{}, ErrStockNotFound
	}
	if err != nil {
		return Stock{}, err
	}
	return st, nil
}

func (s *StockStore) List() ([]Stock, error) {
	rows, err := s.db.Query(`SELECT ` + stockColumns + ` FROM stocks ORDER BY book_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stocks := []Stock{}
	for rows.Next() {
		st, err := scanStock(rows)
		if err != nil {
			return nil, err
		}
		stocks = append(stocks, st)
	}
	return stocks, rows.Err()
}

// SetQuantity overwrites the stock quantity for a book.
func (s *StockStore) SetQuantity(bookId, quantity int64) (Stock, error) {
	st, err := scanStock(s.db.QueryRow(
		`UPDATE stocks SET quantity = ?, updated_at = ? WHERE book_id = ? RETURNING `+stockColumns,
		quantity, time.Now().UTC(), bookId,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Stock{}, ErrStockNotFound
	}
	if err != nil {
		return Stock{}, err
	}
	return st, nil
}

// Increase adds amount to the book's stock.
func (s *StockStore) Increase(bookId, amount int64) (Stock, error) {
	st, err := scanStock(s.db.QueryRow(
		`UPDATE stocks SET quantity = quantity + ?, updated_at = ? WHERE book_id = ? RETURNING `+stockColumns,
		amount, time.Now().UTC(), bookId,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Stock{}, ErrStockNotFound
	}
	if err != nil {
		return Stock{}, err
	}
	return st, nil
}

// Decrease removes amount from the book's stock. The availability check and the
// decrement happen in one statement, so concurrent callers cannot oversell.
// Returns ErrInsufficientStock if fewer than amount units are available.
func (s *StockStore) Decrease(bookId, amount int64) (Stock, error) {
	st, err := scanStock(s.db.QueryRow(
		`UPDATE stocks SET quantity = quantity - ?, updated_at = ?
		 WHERE book_id = ? AND quantity >= ? RETURNING `+stockColumns,
		amount, time.Now().UTC(), bookId, amount,
	))
	if errors.Is(err, sql.ErrNoRows) {
		// No row matched: either there is no stock record, or not enough quantity.
		if _, getErr := s.GetByBookId(bookId); getErr != nil {
			return Stock{}, getErr
		}
		return Stock{}, ErrInsufficientStock
	}
	if err != nil {
		return Stock{}, err
	}
	return st, nil
}

// HasQuantity reports whether at least quantity units of the book are in stock,
// along with the current stock record.
func (s *StockStore) HasQuantity(bookId, quantity int64) (bool, Stock, error) {
	st, err := s.GetByBookId(bookId)
	if err != nil {
		return false, Stock{}, err
	}
	return st.Quantity >= quantity, st, nil
}

func (s *StockStore) Delete(bookId int64) error {
	result, err := s.db.Exec(`DELETE FROM stocks WHERE book_id = ?`, bookId)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrStockNotFound
	}
	return nil
}
