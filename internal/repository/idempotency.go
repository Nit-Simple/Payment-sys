package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"payments-engine/internal/domain"
)

type IdempotencyRepository struct {
	db *pgxpool.Pool
}

func NewIdempotencyRepository(db *pgxpool.Pool) *IdempotencyRepository {
	return &IdempotencyRepository{db: db}
}

func (r *IdempotencyRepository) InsertKey(ctx context.Context, key *domain.IdempotencyKey) (*domain.IdempotencyKey, error) {
	query := `
    INSERT INTO idempotency_keys (
        key, request_hash, status, created_at, expires_at
    ) VALUES (
        $1, $2, 'pending', $3, $4
    )
    ON CONFLICT (key) DO NOTHING
    RETURNING key, payment_id, request_hash, status, response_status, response_body, created_at, expires_at`

	existing := &domain.IdempotencyKey{}

	err := r.db.QueryRow(ctx, query,
		key.Key,
		key.RequestHash,
		key.CreatedAt,
		key.ExpiresAt,
	).Scan(
		&existing.Key,
		scanNullableString(&existing.PaymentID),
		&existing.RequestHash,
		&existing.Status,
		scanNullableInt(&existing.ResponseStatus),
		scanNullableBytes(&existing.ResponseBody),
		&existing.CreatedAt,
		&existing.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrIdempotencyKeyExists
		}
		return nil, fmt.Errorf("insert idempotency key: %w", err)
	}

	return existing, nil
}

func (r *IdempotencyRepository) GetKey(ctx context.Context, key string) (*domain.IdempotencyKey, error) {
	query := `
		SELECT key, payment_id, request_hash, status, response_status, response_body, created_at, expires_at
		FROM idempotency_keys
		WHERE key = $1`

	k := &domain.IdempotencyKey{}

	err := r.db.QueryRow(ctx, query, key).Scan(
		&k.Key,
		scanNullableString(&k.PaymentID),
		&k.RequestHash,
		&k.Status,
		scanNullableInt(&k.ResponseStatus),
		scanNullableBytes(&k.ResponseBody),
		&k.CreatedAt,
		&k.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get idempotency key: %w", err)
	}

	return k, nil
}

func (r *IdempotencyRepository) UpdateKey(ctx context.Context, key string, paymentID string, status string, responseStatus int, responseBody []byte) error {
	query := `
		UPDATE idempotency_keys
		SET
			payment_id      = $1,
			status          = $2,
			response_status = $3,
			response_body   = $4
		WHERE key = $5`

	_, err := r.db.Exec(ctx, query,
		nullableString(paymentID),
		status,
		responseStatus,
		responseBody,
		key,
	)
	if err != nil {
		return fmt.Errorf("update idempotency key: %w", err)
	}

	return nil
}
