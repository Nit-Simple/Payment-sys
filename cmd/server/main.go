package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"payments-engine/internal/config"
	"payments-engine/internal/handler"
	"payments-engine/internal/metrics"
	"payments-engine/internal/recovery"
	"payments-engine/internal/repository"
	"payments-engine/internal/service"
	"payments-engine/pkg/logger"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Environment)
	slog.SetDefault(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := repository.Connect(ctx, cfg)
	if err != nil {
		log.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}

	idempotencyRepo := repository.NewIdempotencyRepository(db)
	idempSvc := service.NewIdempotencyService(idempotencyRepo)
	paymentRepo := repository.NewPaymentRepository(db)
	reconWorker := recovery.NewWorker(db, log)
	go reconWorker.RunReconciliationWorker(ctx)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := db.Stat()
				metrics.DBConnectionsInUse.Set(float64(stats.AcquiredConns()))
				metrics.DBConnectionsIdle.Set(float64(stats.IdleConns()))
			case <-ctx.Done():
				return
			}
		}
	}()

	paymentService := service.NewPaymentService(paymentRepo, cfg.EncryptionKey)

	if err := recovery.RunStartupSweep(ctx, db, log); err != nil {
		log.Error("startup sweep failed", "err", err)
	}

	server := handler.NewServer(cfg, db, log, paymentService, idempSvc)
	if err := server.Start(); err != nil {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
}
