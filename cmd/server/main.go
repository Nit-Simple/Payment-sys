package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"payments-engine/internal/config"
	"payments-engine/internal/handler"
	"payments-engine/internal/repository"
	"payments-engine/internal/service"
	"payments-engine/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Environment)
	slog.SetDefault(log)

	db, err := repository.Connect(context.Background(), cfg)
	if err != nil {
		log.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	paymentRepo := repository.NewPaymentRepository(db)
	paymentService := service.NewPaymentService(paymentRepo, cfg.EncryptionKey)

	server := handler.NewServer(cfg, db, log, paymentService)
	if err := server.Start(); err != nil {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
}
