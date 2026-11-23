package store

import (
	"database/sql"
	"fmt"

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
			id                 INTEGER PRIMARY KEY AUTOINCREMENT,
			customer_firstname TEXT     NOT NULL,
			customer_lastname  TEXT     NOT NULL DEFAULT '',
			customer_email     TEXT     NOT NULL,
			customer_phone     TEXT     NOT NULL,
			notes              TEXT     NOT NULL DEFAULT '',
			status             TEXT     NOT NULL DEFAULT 'placed'
			                   CHECK (status IN ('placed', 'paid', 'dispatched', 'cancelled')),
			total_cents        INTEGER  NOT NULL CHECK (total_cents >= 0),
			placed_at          DATETIME NOT NULL,
			paid_at            DATETIME,
			dispatched_at      DATETIME,
			cancelled_at       DATETIME,
			updated_at         DATETIME NOT NULL
		)`)
	if err != nil {
		return err
	}

	if err := upgradeOrdersCustomerName(db); err != nil {
		return fmt.Errorf("upgrading orders table: %w", err)
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

	if err := dropLegacyTransactionsTable(db); err != nil {
		return fmt.Errorf("upgrading transactions table: %w", err)
	}

	// A transaction is created as 'pending' when a payment request is submitted to
	// Pesapal. The nullable columns are filled in later from the transaction status
	// endpoint / IPN.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id                         INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id                   INTEGER  NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
			merchant_reference         TEXT     NOT NULL UNIQUE,
			order_tracking_id          TEXT     NOT NULL UNIQUE,
			status                     TEXT     NOT NULL DEFAULT 'pending'
			                           CHECK (status IN ('pending', 'completed', 'failed', 'reversed', 'invalid')),
			status_code                INTEGER,
			payment_status_description TEXT,
			confirmation_code          TEXT,
			payment_method             TEXT,
			payment_account            TEXT,
			amount_cents               INTEGER  NOT NULL CHECK (amount_cents >= 0),
			currency                   TEXT     NOT NULL,
			paid_amount_cents          INTEGER  CHECK (paid_amount_cents IS NULL OR paid_amount_cents >= 0),
			paid_currency              TEXT,
			created_at                 DATETIME NOT NULL,
			updated_at                 DATETIME NOT NULL
		)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_transactions_order_id ON transactions(order_id)`)
	if err != nil {
		return err
	}

	return nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM pragma_table_info(?) WHERE name = ?)`, table, column,
	).Scan(&exists)
	return exists, err
}

// upgradeOrdersCustomerName converts databases created with the old single
// customer_name column. Existing names are kept as the first name.
func upgradeOrdersCustomerName(db *sql.DB) error {
	legacy, err := columnExists(db, "orders", "customer_name")
	if err != nil || !legacy {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`ALTER TABLE orders RENAME COLUMN customer_name TO customer_firstname`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE orders ADD COLUMN customer_lastname TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	return tx.Commit()
}

// dropLegacyTransactionsTable removes a transactions table created with the old
// schema so it can be recreated. It refuses to drop a table that holds rows.
func dropLegacyTransactionsTable(db *sql.DB) error {
	var tableExists bool
	err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'transactions')`,
	).Scan(&tableExists)
	if err != nil || !tableExists {
		return err
	}

	upToDate, err := columnExists(db, "transactions", "merchant_reference")
	if err != nil || upToDate {
		return err
	}

	var rows int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&rows); err != nil {
		return err
	}
	if rows > 0 {
		return fmt.Errorf("transactions table uses an old schema and has %d row(s); migrate it manually", rows)
	}

	_, err = db.Exec(`DROP TABLE transactions`)
	return err
}
