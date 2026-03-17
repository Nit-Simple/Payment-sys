package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"payments-engine/internal/config"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	httpServer *http.Server
	db         *pgxpool.Pool
	config     *config.Config
	logger     *slog.Logger
}

func NewServer(cfg *config.Config, db *pgxpool.Pool, log *slog.Logger) *Server {
	s := &Server{
		db:     db,
		config: cfg,
		logger: log,
	}

	s.httpServer = &http.Server{
		Addr:              cfg.Addr(),
		Handler:           s.routes(),
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
	}

	return s
}

func (s *Server) Start() error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("server starting", "addr", s.config.Addr())
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		s.logger.Info("shutdown signal received", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	s.db.Close()

	s.logger.Info("server shutdown complete")
	return nil
}
