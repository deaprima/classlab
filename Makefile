.PHONY: up down migrate migrate-down gen lint test seed dev-api dev-web

up:
	docker compose up -d

down:
	docker compose down

migrate:
	docker compose exec -T postgres psql -U classlab -d classlab -f - < api/db/migrations/000001_init_schema.up.sql

migrate-down:
	docker compose exec -T postgres psql -U classlab -d classlab -f - < api/db/migrations/000001_init_schema.down.sql

gen:
	cd web && pnpm run gen:api

lint:
	cd api && go vet ./...
	cd web && pnpm run lint && pnpm run format:check

test:
	cd api && go test -v ./...
	cd web && pnpm run test

seed:
	@echo "Seeding data will run in Phase 1..."

dev-api:
	cd api && go run ./cmd/server

dev-web:
	cd web && pnpm run dev
