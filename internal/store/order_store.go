package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type OrderStatus string

const (
	StatusPlaced     OrderStatus = "placed"
	StatusPaid       OrderStatus = "paid"
	StatusDispatched OrderStatus = "dispatched"
	StatusCancelled  OrderStatus = "cancelled"
)

// allowedTransitions is the order lifecycle:
//
//	placed -> paid -> dispatched
//	placed -> cancelled
//	paid   -> cancelled
//
// dispatched and cancelled are final. Stock is only reduced on dispatch, and a
// dispatched order cannot be cancelled, so cancelling never has to restock.
var allowedTransitions = map[OrderStatus][]OrderStatus{
	StatusPlaced:     {StatusPaid, StatusCancelled},
	StatusPaid:       {StatusDispatched, StatusCancelled},
	StatusDispatched: {},
	StatusCancelled:  {},
}

// statusTimestampColumn is the column stamped when an order enters that status.
var statusTimestampColumn = map[OrderStatus]string{
	StatusPaid:       "paid_at",
	StatusDispatched: "dispatched_at",
	StatusCancelled:  "cancelled_at",
}

func (s OrderStatus) Valid() bool {
	_, ok := allowedTransitions[s]
	return ok
}

// AllowedNext returns the statuses an order in status s can move to.
func (s OrderStatus) AllowedNext() []OrderStatus {
	return allowedTransitions[s]
}

func (s OrderStatus) CanTransitionTo(next OrderStatus) bool {
	for _, allowed := range allowedTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

type Order struct {
	Id            int64
	CustomerName  string
	CustomerEmail string
	CustomerPhone string
	Notes         string
	Status        OrderStatus
	TotalCents    int64
	PlacedAt      time.Time
	PaidAt        *time.Time
	DispatchedAt  *time.Time
	CancelledAt   *time.Time
	UpdatedAt     time.Time
	Items         []OrderItem
}

type OrderItem struct {
	Id             int64
	OrderId        int64
	BookId         int64
	Quantity       int64
	UnitPriceCents int64
	CreatedAt      time.Time
}

func (i OrderItem) LineTotalCents() int64 {
	return i.Quantity * i.UnitPriceCents
}

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrOrderStatusChanged = errors.New("order status was changed by another request")
)

type InvalidTransitionError struct {
	From OrderStatus
	To   OrderStatus
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("cannot change order status from %s to %s", e.From, e.To)
}

type StockShortage struct {
	BookId    int64
	Requested int64
	Available int64
}

type InsufficientStockError struct {
	Shortages []StockShortage
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("insufficient stock for %d book(s)", len(e.Shortages))
}

func (e *InsufficientStockError) Is(target error) bool {
	return target == ErrInsufficientStock
}

type BookNotFoundError struct {
	BookId int64
}

func (e *BookNotFoundError) Error() string {
	return fmt.Sprintf("book %d not found", e.BookId)
}

func (e *BookNotFoundError) Is(target error) bool {
	return target == BookNotFound
}

type OrderStore struct {
	db *sql.DB
}

func NewOrderStore(db *sql.DB) *OrderStore {
	return &OrderStore{db: db}
}

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	queryRower
	Query(query string, args ...any) (*sql.Rows, error)
}

const orderColumns = `id, customer_name, customer_email, customer_phone, notes, status,
	total_cents, placed_at, paid_at, dispatched_at, cancelled_at, updated_at`

func scanOrder(row rowScanner) (Order, error) {
	var o Order
	var paidAt, dispatchedAt, cancelledAt sql.NullTime
	err := row.Scan(&o.Id, &o.CustomerName, &o.CustomerEmail, &o.CustomerPhone, &o.Notes, &o.Status,
		&o.TotalCents, &o.PlacedAt, &paidAt, &dispatchedAt, &cancelledAt, &o.UpdatedAt)
	o.PaidAt = nullTimePtr(paidAt)
	o.DispatchedAt = nullTimePtr(dispatchedAt)
	o.CancelledAt = nullTimePtr(cancelledAt)
	return o, err
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func (s *OrderStore) Create(o *Order) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var total int64
	var shortages []StockShortage
	for i := range o.Items {
		item := &o.Items[i]

		var available int64
		err := tx.QueryRow(`
			SELECT b.price_cents, COALESCE(s.quantity, 0)
			FROM books b LEFT JOIN stocks s ON s.book_id = b.id
			WHERE b.id = ?`, item.BookId,
		).Scan(&item.UnitPriceCents, &available)
		if errors.Is(err, sql.ErrNoRows) {
			return &BookNotFoundError{BookId: item.BookId}
		}
		if err != nil {
			return err
		}

		if available < item.Quantity {
			shortages = append(shortages, StockShortage{
				BookId: item.BookId, Requested: item.Quantity, Available: available,
			})
		}
		total += item.LineTotalCents()
	}
	if len(shortages) > 0 {
		return &InsufficientStockError{Shortages: shortages}
	}

	now := time.Now().UTC()
	o.Status = StatusPlaced
	o.TotalCents = total
	o.PlacedAt = now
	o.UpdatedAt = now

	result, err := tx.Exec(`
		INSERT INTO orders (customer_name, customer_email, customer_phone, notes, status,
		                    total_cents, placed_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		o.CustomerName, o.CustomerEmail, o.CustomerPhone, o.Notes, o.Status,
		o.TotalCents, o.PlacedAt, o.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if o.Id, err = result.LastInsertId(); err != nil {
		return err
	}

	for i := range o.Items {
		item := &o.Items[i]
		item.OrderId = o.Id
		item.CreatedAt = now
		result, err := tx.Exec(`
			INSERT INTO order_items (order_id, book_id, quantity, unit_price_cents, created_at)
			VALUES (?, ?, ?, ?, ?)`,
			item.OrderId, item.BookId, item.Quantity, item.UnitPriceCents, item.CreatedAt,
		)
		if err != nil {
			return err
		}
		if item.Id, err = result.LastInsertId(); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *OrderStore) GetById(id int64) (Order, error) {
	return getOrder(s.db, id)
}

func getOrder(q querier, id int64) (Order, error) {
	o, err := scanOrder(q.QueryRow(`SELECT `+orderColumns+` FROM orders WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}

	o.Items, err = getOrderItems(q, id)
	if err != nil {
		return Order{}, err
	}
	return o, nil
}

func getOrderItems(q querier, orderId int64) ([]OrderItem, error) {
	rows, err := q.Query(`
		SELECT id, order_id, book_id, quantity, unit_price_cents, created_at
		FROM order_items WHERE order_id = ? ORDER BY id`, orderId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []OrderItem{}
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.Id, &it.OrderId, &it.BookId, &it.Quantity, &it.UnitPriceCents, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *OrderStore) List(status OrderStatus) ([]Order, error) {
	query := `SELECT ` + orderColumns + ` FROM orders`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY id DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (s *OrderStore) UpdateStatus(id int64, next OrderStatus) (Order, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()

	var current OrderStatus
	err = tx.QueryRow(`SELECT status FROM orders WHERE id = ?`, id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}
	if !current.CanTransitionTo(next) {
		return Order{}, &InvalidTransitionError{From: current, To: next}
	}

	// "AND status = ?" makes the update conditional on the status we just read,
	// so two concurrent requests cannot both apply a transition (e.g. a double
	// dispatch reducing stock twice).
	now := time.Now().UTC()
	result, err := tx.Exec(
		`UPDATE orders SET status = ?, `+statusTimestampColumn[next]+` = ?, updated_at = ?
		 WHERE id = ? AND status = ?`,
		next, now, now, id, current,
	)
	if err != nil {
		return Order{}, err
	}
	if n, err := result.RowsAffected(); err != nil {
		return Order{}, err
	} else if n == 0 {
		return Order{}, ErrOrderStatusChanged
	}

	if next == StatusDispatched {
		if err := dispatchStock(tx, id); err != nil {
			return Order{}, err
		}
	}

	order, err := getOrder(tx, id)
	if err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	return order, nil
}

// dispatchStock reduces stock for every item in the order. It checks all items
// before returning so the error lists every shortage, not just the first.
// The caller rolls back the transaction on error.
func dispatchStock(tx *sql.Tx, orderId int64) error {
	items, err := getOrderItems(tx, orderId)
	if err != nil {
		return err
	}

	var shortages []StockShortage
	for _, item := range items {
		_, err := decreaseStock(tx, item.BookId, item.Quantity)
		switch {
		case err == nil:
			continue
		case errors.Is(err, ErrStockNotFound):
			shortages = append(shortages, StockShortage{BookId: item.BookId, Requested: item.Quantity})
		case errors.Is(err, ErrInsufficientStock):
			st, getErr := getStock(tx, item.BookId)
			if getErr != nil {
				return getErr
			}
			shortages = append(shortages, StockShortage{
				BookId: item.BookId, Requested: item.Quantity, Available: st.Quantity,
			})
		default:
			return err
		}
	}
	if len(shortages) > 0 {
		return &InsufficientStockError{Shortages: shortages}
	}
	return nil
}
