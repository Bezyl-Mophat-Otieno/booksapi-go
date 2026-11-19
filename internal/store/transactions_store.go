package store

import (
	"database/sql"
	"errors"
	"time"
)

type Transaction struct {
	Id               int64     `json:"id"`
	OrderID          int64     `json:"order_id"`
	OrderTrackingID  string    `json:"order_tracking_id"`
	ConfirmationCode string    `json:"confirmation_code"`
	StatusCode       string    `json:"status_code"`
	PaymentMethod    string    `json:"payment_method"`
	AmountCents      int64     `json:"amount_cents"`
	Currency         string    `json:"currency"`
	CreatedAt        time.Time `json:"created_at"`
}

var ErrTransactionNotFound = errors.New("transaction not found")

type TransactionStore struct {
	db *sql.DB
}

func NewTransactionStore(db *sql.DB) *TransactionStore {
	return &TransactionStore{db: db}
}

const transactionColumns = `id, order_id, order_tracking_id, confirmation_code, status_code,
	payment_method, amount_cents, currency, created_at`

func scanTransaction(row rowScanner) (Transaction, error) {
	var t Transaction
	err := row.Scan(&t.Id, &t.OrderID, &t.OrderTrackingID, &t.ConfirmationCode, &t.StatusCode,
		&t.PaymentMethod, &t.AmountCents, &t.Currency, &t.CreatedAt)
	return t, err
}

func scanTransactions(rows *sql.Rows) ([]Transaction, error) {
	defer rows.Close()
	transactions := []Transaction{}
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}
	return transactions, rows.Err()
}

func orderExists(q queryRower, orderId int64) (bool, error) {
	var exists bool
	err := q.QueryRow(`SELECT EXISTS(SELECT 1 FROM orders WHERE id = ?)`, orderId).Scan(&exists)
	return exists, err
}

// Create records a transaction against an existing order.
// Returns ErrOrderNotFound if the order does not exist.
func (s *TransactionStore) Create(t *Transaction) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	exists, err := orderExists(tx, t.OrderID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrOrderNotFound
	}

	t.CreatedAt = time.Now().UTC()
	result, err := tx.Exec(`
		INSERT INTO transactions (order_id, order_tracking_id, confirmation_code, status_code,
		                          payment_method, amount_cents, currency, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.OrderID, t.OrderTrackingID, t.ConfirmationCode, t.StatusCode,
		t.PaymentMethod, t.AmountCents, t.Currency, t.CreatedAt,
	)
	if err != nil {
		return err
	}
	if t.Id, err = result.LastInsertId(); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *TransactionStore) GetById(id int64) (Transaction, error) {
	t, err := scanTransaction(s.db.QueryRow(`SELECT `+transactionColumns+` FROM transactions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, ErrTransactionNotFound
	}
	if err != nil {
		return Transaction{}, err
	}
	return t, nil
}

// List returns transactions newest first. If trackingId is not empty, only
// transactions with that order_tracking_id are returned.
func (s *TransactionStore) List(trackingId string) ([]Transaction, error) {
	query := `SELECT ` + transactionColumns + ` FROM transactions`
	args := []any{}
	if trackingId != "" {
		query += ` WHERE order_tracking_id = ?`
		args = append(args, trackingId)
	}
	query += ` ORDER BY id DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return scanTransactions(rows)
}

// ListByOrder returns an order's transactions newest first.
// Returns ErrOrderNotFound if the order does not exist, so callers can tell a
// missing order apart from an order with no transactions yet.
func (s *TransactionStore) ListByOrder(orderId int64) ([]Transaction, error) {
	exists, err := orderExists(s.db, orderId)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrOrderNotFound
	}

	rows, err := s.db.Query(
		`SELECT `+transactionColumns+` FROM transactions WHERE order_id = ? ORDER BY id DESC`, orderId)
	if err != nil {
		return nil, err
	}
	return scanTransactions(rows)
}
