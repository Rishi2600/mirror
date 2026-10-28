// Package dbtest gives integration tests a migrated database.
package dbtest

import (
	"context"
	"os"
	"testing"

	"gorm.io/gorm"

	"mirror/server/internal/db"
)

// Tx returns a transaction on the database at DATABASE_URL, with all
// migrations applied. It is rolled back when the test ends, so tests leave
// no data behind.
func Tx(t *testing.T) *gorm.DB {
	t.Helper()
	conn := open(t)
	tx := conn.Begin()
	t.Cleanup(func() {
		tx.Rollback()
		db.Close(conn)
	})
	return tx
}

// Conn returns a plain connection pool, for tests that need several
// transactions at once (such as two swipes racing). Its writes are real:
// the test must remove its own rows.
func Conn(t *testing.T) *gorm.DB {
	t.Helper()
	conn := open(t)
	t.Cleanup(func() { db.Close(conn) })
	return conn
}

func open(t *testing.T) *gorm.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL is not set; set it in server/.env (see .env.example)")
	}
	conn, err := db.Open(url)
	if err != nil {
		t.Fatalf("cannot reach Postgres (run make db-up): %v", err)
	}
	if _, err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return conn
}
