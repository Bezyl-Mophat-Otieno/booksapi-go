package store

import (
	"database/sql"
	"errors"
	"time"
)

type TransactionStatus string

const (
	TransactionPending   TransactionStatus = "pending"
	TransactionCompleted TransactionStatus = "completed"
	TransactionFailed    TransactionStatus = "failed"
	TransactionReversed  TransactionStatus = "reversed"
	TransactionInvalid   TransactionStatus = "invalid"
)


type Transaction struct {
	Id                       int64
	OrderID                  int64
	MerchantReference        string
	OrderTrackingID          string
	Status                   TransactionStatus
	StatusCode               *int64
	PaymentStatusDescription *string
	ConfirmationCode         *string
	PaymentMethod            *string
	PaymentAccount           *string
	AmountCents              int64
	Currency                 string
	PaidAmountCents          *int64
	PaidCurrency             *string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

var ErrTransactionNotFound = errors.New("transaction not found")

type TransactionStore struct {
	db *sql.DB
}

func NewTransactionStore(db *sql.DB) *TransactionStore {
	return &TransactionStore{db: db}
}

const transactionColumns = `id, order_id, merchant_reference, order_tracking_id, status, status_code,
	payment_status_description, confirmation_code, payment_method, payment_account,
	amount_cents, currency, paid_amount_cents, paid_currency, created_at, updated_at`

func scanTransaction(row rowScanner) (Transaction, error) {
	var t Transaction
	err := row.Scan(&t.Id, &t.OrderID, &t.MerchantReference, &t.OrderTrackingID, &t.Status, &t.StatusCode,
		&t.PaymentStatusDescription, &t.ConfirmationCode, &t.PaymentMethod, &t.PaymentAccount,
		&t.AmountCents, &t.Currency, &t.PaidAmountCents, &t.PaidCurrency, &t.CreatedAt, &t.UpdatedAt)
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

	if t.Status == "" {
		t.Status = TransactionPending
	}
	t.CreatedAt = time.Now().UTC()
	t.UpdatedAt = t.CreatedAt
	result, err := tx.Exec(`
		INSERT INTO transactions (order_id, merchant_reference, order_tracking_id, status, status_code,
		                          payment_status_description, confirmation_code, payment_method, payment_account,
		                          amount_cents, currency, paid_amount_cents, paid_currency, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.OrderID, t.MerchantReference, t.OrderTrackingID, t.Status, t.StatusCode,
		t.PaymentStatusDescription, t.ConfirmationCode, t.PaymentMethod, t.PaymentAccount,
		t.AmountCents, t.Currency, t.PaidAmountCents, t.PaidCurrency, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if t.Id, err = result.LastInsertId(); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *TransactionStore) CountByOrder(orderId int64) (int64, error) {
	var count int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE order_id = ?`, orderId).Scan(&count)
	return count, err
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
