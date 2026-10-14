.PHONY: up down migrate migrate-down migrate-force gen lint test seed dev-api dev-web

up:
	docker compose up -d

down:
	docker compose down

# golang-migrate runs in Docker, sharing the Postgres container's network, and tracks versions in schema_migrations.
MIGRATE = docker run --rm --network container:classlab-postgres -v "$(CURDIR)/api/db/migrations:/migrations" migrate/migrate:v4.18.3 -path=/migrations -database "postgres://classlab:classlab_secret@localhost:5432/classlab?sslmode=disable"

migrate:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

# One-time, for a database created by the old psql-based target: make migrate-force V=1
migrate-force:
	$(MIGRATE) force $(V)

gen:
	docker run --rm -v "$(CURDIR)/api:/src" -w /src sqlc/sqlc:1.30.0 generate
	cd web && pnpm run gen:api

lint:
	cd api && go vet ./...
	cd web && pnpm run lint && pnpm run format:check

test:
	cd api && go test -v ./...
	cd web && pnpm run test

seed:
	cd api && go run ./cmd/seed

dev-api:
	cd api && go run ./cmd/server

dev-web:
	cd web && pnpm run dev
