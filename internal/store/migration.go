package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			title      TEXT     NOT NULL,
			author     TEXT     NOT NULL,
			pages      INTEGER  NOT NULL DEFAULT 0,
			price_cents INTEGER  NOT NULL DEFAULT 0 CHECK (price_cents >= 0),
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS stocks (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id    INTEGER  NOT NULL UNIQUE REFERENCES books(id) ON DELETE CASCADE,
			quantity   INTEGER  NOT NULL DEFAULT 0 CHECK (quantity >= 0),
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			customer_name  TEXT     NOT NULL,
			customer_email TEXT     NOT NULL,
			customer_phone TEXT     NOT NULL,
			notes          TEXT     NOT NULL DEFAULT '',
			status         TEXT     NOT NULL DEFAULT 'placed'
			               CHECK (status IN ('placed', 'paid', 'dispatched', 'cancelled')),
			total_cents    INTEGER  NOT NULL CHECK (total_cents >= 0),
			placed_at      DATETIME NOT NULL,
			paid_at        DATETIME,
			dispatched_at  DATETIME,
			cancelled_at   DATETIME,
			updated_at     DATETIME NOT NULL
		)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS order_items (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id         INTEGER  NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			book_id          INTEGER  NOT NULL REFERENCES books(id)  ON DELETE RESTRICT,
			quantity         INTEGER  NOT NULL CHECK (quantity > 0),
			unit_price_cents INTEGER  NOT NULL CHECK (unit_price_cents >= 0),
			created_at       DATETIME NOT NULL,
			UNIQUE (order_id, book_id)
		)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`)

	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id         INTEGER  NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
			order_tracking_id TEXT     NOT NULL,
			confirmation_code TEXT     NOT NULL,
			status_code       TEXT     NOT NULL,
			payment_method    TEXT     NOT NULL,
			amount_cents      INTEGER  NOT NULL CHECK (amount_cents >= 0),
			currency          TEXT     NOT NULL,
			created_at        DATETIME NOT NULL
		)`)
	if err != nil {
		return err
	}

	return nil
}
