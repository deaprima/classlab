param(
    [Parameter(Position=0, Mandatory=$true)]
    [string]$Action,
    [Parameter(Position=1)]
    [string]$Arg
)

# golang-migrate runs in Docker, sharing the Postgres container network; versions live in schema_migrations.
function Invoke-Migrate([string[]]$MigrateArgs) {
    docker run --rm --network container:classlab-postgres -v "${PWD}/api/db/migrations:/migrations" migrate/migrate:v4.18.3 -path=/migrations -database "postgres://classlab:classlab_secret@localhost:5432/classlab?sslmode=disable" @MigrateArgs
}

switch ($Action) {
    "up" {
        docker compose up -d
    }
    "down" {
        docker compose down
    }
    "migrate" {
        Invoke-Migrate @("up")
    }
    "migrate-down" {
        Invoke-Migrate @("down", "1")
    }
    "migrate-force" {
        # One-time, for a database created by the old psql-based target: task.ps1 migrate-force 1
        Invoke-Migrate @("force", $Arg)
    }
    "gen" {
        docker run --rm -v "${PWD}/api:/src" -w /src sqlc/sqlc:1.30.0 generate
        Set-Location web
        pnpm run gen:api
        Set-Location ..
    }
    "lint" {
        Write-Host "Linting backend..." -ForegroundColor Cyan
        Set-Location api
        go vet ./...
        Set-Location ..

        Write-Host "Linting frontend..." -ForegroundColor Cyan
        Set-Location web
        pnpm run lint
        pnpm run format:check
        Set-Location ..
    }
    "test" {
        Write-Host "Testing backend..." -ForegroundColor Cyan
        Set-Location api
        go test -v ./...
        Set-Location ..

        Write-Host "Testing frontend..." -ForegroundColor Cyan
        Set-Location web
        pnpm run test
        Set-Location ..
    }
    "seed" {
        Set-Location api
        go run ./cmd/seed
        Set-Location ..
    }
    "dev-api" {
        Set-Location api
        go run ./cmd/server
        Set-Location ..
    }
    "dev-web" {
        Set-Location web
        pnpm run dev
        Set-Location ..
    }
    default {
        Write-Host "Aksi tidak dikenal: $Action" -ForegroundColor Red
        Write-Host "Daftar aksi: up, down, migrate, migrate-down, migrate-force, gen, seed, lint, test, dev-api, dev-web"
    }
}
