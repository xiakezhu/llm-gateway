package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateSQLiteCreatesAPIKeysTable(t *testing.T) {
	ctx := context.Background()
	database, err := OpenSQLite(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if err := MigrateSQLite(ctx, database); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	var tableName string
	err = database.QueryRowContext(
		ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'api_keys'`,
	).Scan(&tableName)
	if err != nil {
		t.Fatalf("query sqlite master: %v", err)
	}
	if tableName != "api_keys" {
		t.Fatalf("expected api_keys table, got %s", tableName)
	}
}
