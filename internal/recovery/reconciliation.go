package recovery

import (
	"context"
	"fmt"
	"log/slog"
	"payments-engine/internal/metrics"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	workerMinAge   = 5 * time.Minute // older than this = genuinely stuck
	workerLimit    = 100
	workerInterval = 5 * time.Minute
)

type Worker struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewWorker(db *pgxpool.Pool, logger *slog.Logger) *Worker {
	return &Worker{db: db, logger: logger}
}

func (w *Worker) RunReconciliationWorker(ctx context.Context) {
	w.logger.InfoContext(ctx, "reconciliation worker: started",
		"interval", workerInterval,
		"min_age", workerMinAge,
		"limit", workerLimit,
	)
	w.sweep(ctx)
	ticker := time.NewTicker(workerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}
func fetchAllStuckKeys(ctx context.Context, db *pgxpool.Pool) ([]stuckKey, error) {
	query := `
        SELECT key, COALESCE(payment_id::text, '')
        FROM idempotency_keys
        WHERE status = 'pending'
        AND created_at < now() - $1::interval
        ORDER BY created_at ASC
        LIMIT $2
        FOR UPDATE SKIP LOCKED`

	rows, err := db.Query(ctx, query,
		workerMinAge.String(),
		workerLimit,
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

func (w *Worker) sweep(ctx context.Context) {
	start := time.Now()

	keys, err := fetchAllStuckKeys(ctx, w.db)
	if err != nil {
		metrics.RecoveryErrors.Inc()
		w.logger.ErrorContext(ctx, "reconciliation worker: fetch stuck keys failed", "err", err)
		return
	}

	if len(keys) == 0 {
		return
	}

	w.logger.InfoContext(ctx, "reconciliation worker: resolving stuck keys", "count", len(keys))

	var resolved, failed int

	for _, key := range keys {
		if ctx.Err() != nil {
			break
		}

		if err := resolveKey(ctx, w.db, w.logger, key); err != nil {
			metrics.RecoveryErrors.Inc()
			w.logger.ErrorContext(ctx, "reconciliation worker: failed to resolve key",
				"key", key.Key,
				"err", err,
			)
			failed++
			continue
		}

		resolved++
	}

	metrics.RecoverySweepDuration.Observe(time.Since(start).Seconds())

	if resolved > 0 || failed > 0 {
		w.logger.InfoContext(ctx, "reconciliation worker: sweep complete",
			"resolved", resolved,
			"failed", failed,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
