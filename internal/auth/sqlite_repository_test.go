package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"local-llm-gateway/internal/db"
)

func TestSQLiteRepositoryEnsureAndFindAPIKey(t *testing.T) {
	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if err := db.MigrateSQLite(ctx, database); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewSQLiteRepository(database)
	key := APIKey{
		ID:        "key_test",
		Name:      "test",
		KeyPrefix: KeyPrefix("sk-test"),
		KeyHash:   HashAPIKey("sk-test"),
		Status:    APIKeyStatusActive,
		RPMLimit:  30,
		TPMLimit:  3000,
	}

	if err := repo.EnsureAPIKey(ctx, key); err != nil {
		t.Fatalf("ensure api key: %v", err)
	}

	got, err := repo.FindByHash(ctx, key.KeyHash)
	if err != nil {
		t.Fatalf("find api key: %v", err)
	}

	if got.ID != key.ID {
		t.Fatalf("expected id %s, got %s", key.ID, got.ID)
	}
	if got.KeyPrefix != key.KeyPrefix {
		t.Fatalf("expected key prefix %s, got %s", key.KeyPrefix, got.KeyPrefix)
	}
	if got.Status != APIKeyStatusActive {
		t.Fatalf("expected key status active, got %s", got.Status)
	}
	if got.RPMLimit != key.RPMLimit {
		t.Fatalf("expected rpm %d, got %d", key.RPMLimit, got.RPMLimit)
	}
	if got.CreatedAt.IsZero() {
		t.Fatalf("expected created_at to be set")
	}
	if got.UpdatedAt.IsZero() {
		t.Fatalf("expected updated_at to be set")
	}
}

func TestSQLiteRepositoryFindMissingAPIKey(t *testing.T) {
	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if err := db.MigrateSQLite(ctx, database); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewSQLiteRepository(database)
	_, err = repo.FindByHash(ctx, HashAPIKey("sk-missing"))
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestSQLiteRepositoryCreateAPIKey(t *testing.T) {
	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if err := db.MigrateSQLite(ctx, database); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewSQLiteRepository(database)
	key := APIKey{
		ID:        "key_create",
		Name:      "created",
		KeyPrefix: KeyPrefix("sk-create"),
		KeyHash:   HashAPIKey("sk-create"),
	}

	if err := repo.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	got, err := repo.FindByHash(ctx, key.KeyHash)
	if err != nil {
		t.Fatalf("find api key: %v", err)
	}

	if got.Status != APIKeyStatusActive {
		t.Fatalf("expected default status active, got %s", got.Status)
	}
	if got.RPMLimit != 60 {
		t.Fatalf("expected default rpm 60, got %d", got.RPMLimit)
	}
	if got.TPMLimit != 60000 {
		t.Fatalf("expected default tpm 60000, got %d", got.TPMLimit)
	}
}

func TestSQLiteRepositoryCreateAPIKeyConflict(t *testing.T) {
	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if err := db.MigrateSQLite(ctx, database); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewSQLiteRepository(database)
	key := APIKey{
		ID:        "key_create",
		Name:      "created",
		KeyPrefix: KeyPrefix("sk-create"),
		KeyHash:   HashAPIKey("sk-create"),
	}

	if err := repo.CreateAPIKey(ctx, key); err != nil {
		t.Fatalf("create api key: %v", err)
	}
	key.ID = "key_create_duplicate"
	err = repo.CreateAPIKey(ctx, key)
	if !errors.Is(err, ErrAPIKeyConflict) {
		t.Fatalf("expected ErrAPIKeyConflict, got %v", err)
	}
}

func TestSQLiteRepositoryEnsureAPIKeyIgnoresConflict(t *testing.T) {
	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if err := db.MigrateSQLite(ctx, database); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewSQLiteRepository(database)
	key := APIKey{
		ID:        "key_seed",
		Name:      "seed",
		KeyPrefix: KeyPrefix("sk-seed"),
		KeyHash:   HashAPIKey("sk-seed"),
	}

	if err := repo.EnsureAPIKey(ctx, key); err != nil {
		t.Fatalf("ensure api key: %v", err)
	}
	key.ID = "key_seed_duplicate"
	key.Name = "changed"
	if err := repo.EnsureAPIKey(ctx, key); err != nil {
		t.Fatalf("ensure duplicate api key: %v", err)
	}

	got, err := repo.FindByHash(ctx, key.KeyHash)
	if err != nil {
		t.Fatalf("find api key: %v", err)
	}
	if got.ID != "key_seed" {
		t.Fatalf("expected original id to remain, got %s", got.ID)
	}
	if got.Name != "seed" {
		t.Fatalf("expected original name to remain, got %s", got.Name)
	}
}
