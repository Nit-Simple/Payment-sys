package main

import (
	"context"
	"fmt"
	"os"
	"payments-engine/internal/config"
	"payments-engine/internal/handler"
	"payments-engine/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	db, err := repository.Connect(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	server := handler.NewServer(cfg, db)
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
