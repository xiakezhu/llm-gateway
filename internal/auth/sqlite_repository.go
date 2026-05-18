package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) FindByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	const query = `
SELECT id, name, key_hash, rpm_limit, tpm_limit, enabled
FROM api_keys
WHERE key_hash = ?
LIMIT 1`

	var row APIKey
	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
		&row.ID,
		&row.Name,
		&row.KeyHash,
		&row.RPMLimit,
		&row.TPMLimit,
		&row.Enabled,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("query api key by hash: %w", err)
	}

	return &row, nil
}
