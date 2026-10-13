param(
    [Parameter(Position=0, Mandatory=$true)]
    [string]$Action
)

switch ($Action) {
    "up" {
        docker compose up -d
    }
    "down" {
        docker compose down
    }
    "migrate" {
        Get-Content api/db/migrations/000001_init_schema.up.sql -Raw | docker compose exec -T postgres psql -U classlab -d classlab
    }
    "migrate-down" {
        Get-Content api/db/migrations/000001_init_schema.down.sql -Raw | docker compose exec -T postgres psql -U classlab -d classlab
    }
    "gen" {
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
        Write-Host "Daftar aksi: up, down, migrate, migrate-down, gen, lint, test, dev-api, dev-web"
    }
}
