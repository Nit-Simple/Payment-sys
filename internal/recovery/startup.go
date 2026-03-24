package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"payments-engine/internal/domain"
	"payments-engine/internal/metrics"
)

const (
	sweepMinAge = 5 * time.Minute // older than this = genuinely stuck
	sweepMaxAge = 24 * time.Hour  // younger than this = our responsibility
	sweepLimit  = 100             // max keys per sweep
)

func RunStartupSweep(ctx context.Context, db *pgxpool.Pool, logger *slog.Logger) error {
	start := time.Now()

	logger.InfoContext(ctx, "startup sweep: beginning")

	keys, err := fetchStuckKeys(ctx, db)
	if err != nil {
		metrics.RecoveryErrors.Inc()
		return fmt.Errorf("startup sweep: fetch stuck keys: %w", err)
	}

	if len(keys) == 0 {
		logger.InfoContext(ctx, "startup sweep: no stuck keys found")
		metrics.RecoverySweepDuration.Observe(time.Since(start).Seconds())
		return nil
	}

	logger.InfoContext(ctx, "startup sweep: found stuck keys", "count", len(keys))

	var resolved, failed int

	for _, key := range keys {
		if ctx.Err() != nil {
			logger.WarnContext(ctx, "startup sweep: context cancelled, stopping early",
				"resolved", resolved,
				"failed", failed,
			)
			break
		}

		if err := resolveKey(ctx, db, logger, key); err != nil {
			metrics.RecoveryErrors.Inc()
			logger.ErrorContext(ctx, "startup sweep: failed to resolve key",
				"key", key.Key,
				"err", err,
			)
			failed++
			continue
		}

		resolved++
	}

	metrics.RecoverySweepDuration.Observe(time.Since(start).Seconds())

	logger.InfoContext(ctx, "startup sweep: complete",
		"resolved", resolved,
		"failed", failed,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil
}

type stuckKey struct {
	Key       string
	PaymentID string // empty if NULL
}

func fetchStuckKeys(ctx context.Context, db *pgxpool.Pool) ([]stuckKey, error) {
	query := `
		SELECT key, COALESCE(payment_id::text, '')
		FROM idempotency_keys
		WHERE status = 'pending'
		AND created_at < now() - $1::interval
		AND created_at > now() - $2::interval
		ORDER BY created_at ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED`

	rows, err := db.Query(ctx, query,
		sweepMinAge.String(),
		sweepMaxAge.String(),
		sweepLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("query stuck keys: %w", err)
	}
	defer rows.Close()

	var keys []stuckKey
	for rows.Next() {
		var k stuckKey
		if err := rows.Scan(&k.Key, &k.PaymentID); err != nil {
			return nil, fmt.Errorf("scan stuck key: %w", err)
		}
		keys = append(keys, k)
	}

	return keys, rows.Err()
}

func resolveKey(ctx context.Context, db *pgxpool.Pool, logger *slog.Logger, key stuckKey) error {
	if key.PaymentID == "" {
		return markFailed(ctx, db, key.Key)
	}

	// payment exists — fetch it and reconstruct response
	payment, err := fetchPayment(ctx, db, key.PaymentID)
	if err != nil {
		// payment_id on key but payment not found — shouldn't happen
		// foreign key constraint protects this, but handle defensively
		logger.ErrorContext(ctx, "startup sweep: payment not found for key",
			"key", key.Key,
			"payment_id", key.PaymentID,
		)
		return markFailed(ctx, db, key.Key)
	}

	resp := domain.PaymentToResponse(payment)
	body, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	return markCompleted(ctx, db, key.Key, key.PaymentID, body)
}

func fetchPayment(ctx context.Context, db *pgxpool.Pool, id string) (*domain.Payment, error) {
	query := `
		SELECT id, customer_id, amount, currency, status, method,
		       card_last_four, card_brand,
		       upi_id,
		       ifsc_code, account_holder_name,
		       email, created_at
		FROM payments
		WHERE id = $1`

	p := &domain.Payment{}
	err := db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.CustomerID, &p.Amount, &p.Currency, &p.Status, &p.Method,
		&p.CardLastFour, &p.CardBrand,
		&p.UPIID,
		&p.IFSCCode, &p.AccountHolderName,
		&p.Email, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch payment: %w", err)
	}
	return p, nil
}

func markCompleted(ctx context.Context, db *pgxpool.Pool, key, paymentID string, body []byte) error {
	query := `
		UPDATE idempotency_keys
		SET status          = 'completed',
		    payment_id      = $1,
		    response_status = $2,
		    response_body   = $3
		WHERE key    = $4
		AND   status = 'pending'` // guard — don't overwrite if already resolved

	result, err := db.Exec(ctx, query, paymentID, 201, body, key)
	if err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}

	if result.RowsAffected() > 0 {
		metrics.RecoveryKeysResolved.WithLabelValues("completed").Inc()
	}

	return nil
}

func markFailed(ctx context.Context, db *pgxpool.Pool, key string) error {
	errBody, _ := json.Marshal(domain.ErrorResponse{
		Error: "request did not complete — please retry with a new idempotency key",
		Code:  "processing_timeout",
	})

	query := `
		UPDATE idempotency_keys
		SET status          = 'failed',
		    response_status = $1,
		    response_body   = $2
		WHERE key    = $3
		AND   status = 'pending'` // guard

	result, err := db.Exec(ctx, query, 500, errBody, key)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}

	if result.RowsAffected() > 0 {
		metrics.RecoveryKeysResolved.WithLabelValues("failed").Inc()
	}

	return nil
}
