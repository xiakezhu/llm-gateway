package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const MaxKeyPrefixLength = 5

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) FindByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	const query = `
SELECT
    id,
    name,
    key_prefix,
    key_hash,
    rpm_limit,
    tpm_limit,
    status,
    created_at,
    updated_at,
    expires_at,
    disabled_at
FROM api_keys
WHERE key_hash = ?
LIMIT 1`

	var row APIKey
	var expiresAt sql.NullTime
	var disabledAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, keyHash).Scan(
		&row.ID,
		&row.Name,
		&row.KeyPrefix,
		&row.KeyHash,
		&row.RPMLimit,
		&row.TPMLimit,
		&row.Status,
		&row.CreatedAt,
		&row.UpdatedAt,
		&expiresAt,
		&disabledAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("query api key by hash: %w", err)
	}

	if expiresAt.Valid {
		row.ExpiresAt = &expiresAt.Time
	}
	if disabledAt.Valid {
		row.DisabledAt = &disabledAt.Time
	}
	return &row, nil
}

func (r *SQLiteRepository) EnsureAPIKey(ctx context.Context, key APIKey) error {
	const query = `
INSERT INTO api_keys (
    id,
    name,
    key_prefix,
    key_hash,
    rpm_limit,
    tpm_limit,
    status,
    created_at,
    updated_at,
    expires_at,
    disabled_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(key_hash) DO NOTHING`

	key, err := normalizeAPIKeyForInsert(key)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(
		ctx,
		query,
		key.ID,
		key.Name,
		key.KeyPrefix,
		key.KeyHash,
		key.RPMLimit,
		key.TPMLimit,
		key.Status,
		key.CreatedAt,
		key.UpdatedAt,
		key.ExpiresAt,
		key.DisabledAt,
	)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}

	return nil
}

func (r *SQLiteRepository) CreateAPIKey(ctx context.Context, key APIKey) error {
	const query = `
INSERT INTO api_keys (
    id,
    name,
    key_prefix,
    key_hash,
    rpm_limit,
    tpm_limit,
    status,
    created_at,
    updated_at,
    expires_at,
    disabled_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	key, err := normalizeAPIKeyForInsert(key)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(
		ctx,
		query,
		key.ID,
		key.Name,
		key.KeyPrefix,
		key.KeyHash,
		key.RPMLimit,
		key.TPMLimit,
		key.Status,
		key.CreatedAt,
		key.UpdatedAt,
		key.ExpiresAt,
		key.DisabledAt,
	)
	if err != nil {
		if isSQLiteConstraintError(err) {
			return ErrAPIKeyConflict
		}
		return fmt.Errorf("create api key: %w", err)
	}

	return nil
}

func KeyPrefix(rawKey string) string {
	rawKey = strings.TrimSpace(rawKey)
	if len(rawKey) <= MaxKeyPrefixLength {
		return rawKey
	}
	return rawKey[:MaxKeyPrefixLength]
}

func validateAPIKeyStatus(status string) error {
	switch status {
	case APIKeyStatusActive, APIKeyStatusDisabled, APIKeyStatusRevoked, APIKeyStatusExpired:
		return nil
	default:
		return fmt.Errorf("invalid api key status %q", status)
	}
}

func normalizeAPIKeyForInsert(key APIKey) (APIKey, error) {
	if key.ID == "" {
		return APIKey{}, fmt.Errorf("api key id cannot be empty")
	}
	if key.Name == "" {
		return APIKey{}, fmt.Errorf("api key name cannot be empty")
	}
	if key.KeyHash == "" {
		return APIKey{}, fmt.Errorf("api key hash cannot be empty")
	}
	if key.KeyPrefix == "" {
		return APIKey{}, fmt.Errorf("api key prefix cannot be empty")
	}
	if key.RPMLimit == 0 {
		key.RPMLimit = 60
	}
	if key.TPMLimit == 0 {
		key.TPMLimit = 60000
	}
	if key.Status == "" {
		key.Status = APIKeyStatusActive
	}
	if err := validateAPIKeyStatus(key.Status); err != nil {
		return APIKey{}, err
	}

	now := timeNow()
	if key.CreatedAt.IsZero() {
		key.CreatedAt = now
	}
	if key.UpdatedAt.IsZero() {
		key.UpdatedAt = now
	}

	return key, nil
}

func isSQLiteConstraintError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "constraint")
}

var timeNow = func() time.Time {
	return time.Now().UTC()
}
