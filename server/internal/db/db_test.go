package db_test

import (
	"testing"

	"mirror/server/internal/dbtest"
)

func TestMigrationsCreateTables(t *testing.T) {
	tx := dbtest.Tx(t)

	var usersTable, eventsSchema bool
	tx.Raw("SELECT to_regclass('public.users') IS NOT NULL").Scan(&usersTable)
	tx.Raw("SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'events')").Scan(&eventsSchema)

	if !usersTable {
		t.Error("users table is missing")
	}
	if !eventsSchema {
		t.Error("events schema is missing")
	}
}
