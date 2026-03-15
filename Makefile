include .env
export

.PHONY: run build down up

up:
	docker compose up -d postgres

down:
	docker compose down

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server