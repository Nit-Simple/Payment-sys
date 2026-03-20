package repository

import (
	"context"
	"fmt"
	"payments-engine/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database url: %w", err)
	}

	poolCfg.MaxConns = cfg.DBMaxConns
	poolCfg.MinConns = cfg.DBMinConns
	poolCfg.MaxConnIdleTime = cfg.DBMaxConnIdle

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
func scanNullableInt(i *int) any {
	return pgScanInt{i}
}

type pgScanInt struct{ v *int }

func (s pgScanInt) Scan(src any) error {
	if src == nil {
		*s.v = 0
		return nil
	}
	switch v := src.(type) {
	case int64:
		*s.v = int(v)
	case int32:
		*s.v = int(v)
	}
	return nil
}
