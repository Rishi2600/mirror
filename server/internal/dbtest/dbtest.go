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

	tx := conn.Begin()
	t.Cleanup(func() {
		tx.Rollback()
		db.Close(conn)
	})
	return tx
}
