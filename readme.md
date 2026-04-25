# APANT BE

Backend API for APANT using Go and Fiber with clean architecture.

## 1) Prerequisites

- Go version from `go.mod`.
- Docker Engine (required by scanner execution flow).
- PostgreSQL (if using `AUTH_STORAGE=postgres`).

## 2) Local Setup

1. Copy environment template:

```powershell
Copy-Item .env.example .env
```

2. Install dependencies and run:

```powershell
go mod tidy
go run .\cmd\api
```

## 3) Database Migration (PostgreSQL)

Fiber does not provide built-in schema migration like Laravel Artisan.
This project uses SQL migration files under `migrations/` and `golang-migrate`.

Install migration CLI:

```powershell
go install -tags "postgres" github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

Create new migration:

```powershell
migrate create -ext sql -dir migrations -seq add_new_column_to_users
```

Run migration up:

```powershell
migrate -path migrations -database "postgres://postgres:YOUR_PASSWORD@localhost:5432/apant_be?sslmode=disable" up
```

Rollback one migration:

```powershell
migrate -path migrations -database "postgres://postgres:YOUR_PASSWORD@localhost:5432/apant_be?sslmode=disable" down 1
```

Migration rules:

- Always create new migration files for schema changes.
- Never edit old migration files that already ran in shared environments.
- Keep both up and down migrations valid.
- `APP_ENV=production` disables auth `AutoMigrate` by default.

## 4) Docker

Build image:

```powershell
docker build -t apant-be:local .
```

Run image:

```powershell
docker run --rm -p 8080:8080 --env-file .env apant-be:local
```

If scanner endpoints must run from inside this container, provide Docker socket access (high privilege, use only in trusted environments):

```powershell
docker run --rm -p 8080:8080 --env-file .env -v //var/run/docker.sock:/var/run/docker.sock apant-be:local
```

## 5) GitHub CI/CD

This repository includes:

- CI workflow: `.github/workflows/ci.yml`
  - Go format check (`gofmt`)
  - `go vet`
  - `go test ./...`
  - Docker build check (no push)

- CD workflow: `.github/workflows/docker-publish.yml`
  - Trigger on push to `main`, tags `v*`, and manual dispatch
  - Build and push Docker image to GHCR
  - Image metadata tagging (branch, tag, sha, latest on default branch)
  - SBOM and provenance enabled

- Security workflow: `.github/workflows/codeql.yml`
  - Static code analysis for Go on pull request, push to `main`, and weekly schedule
  - Uploads security findings to GitHub Security tab

- Dependency automation: `.github/dependabot.yml`
  - Weekly updates for Go modules and GitHub Actions

### GHCR Setup

1. Ensure GitHub Actions is enabled for the repository.
2. Push to `main` (or create tag `vX.Y.Z`) to publish image.
3. Published image name pattern:

```text
ghcr.io/<owner>/<repo>:<tag>
```

## 6) Security Baseline

- `.env` and all secret variants are ignored by git.
- Use `.env.example` only for safe placeholders.
- Rotate any key that was ever committed or shared.
- Use strong `JWT_SECRET` in production.
- Prefer short-lived tokens and least-privilege credentials.
- Restrict who can push to `main` and require pull request reviews.

## 7) Environment Variables

- `APP_NAME`
- `PORT`
- `APP_ENV`
- `OPENAI_API_KEY`
- `OPENAI_MODEL`
- `ANTHROPIC_API_KEY`
- `ANTHROPIC_MODEL`
- `ALLOWED_ORIGINS`
- `JWT_SECRET`
- `AUTH_TOKEN_TTL_HOURS`
- `AUTH_REFRESH_TOKEN_TTL_HOURS`
- `AUTH_STORAGE` (`memory` or `postgres`)
- `DB_HOST`
- `DB_PORT`
- `DB_USER`
- `DB_PASSWORD`
- `DB_NAME`
- `DB_SSLMODE`
- `DB_TIMEZONE`
- `DOCKER_BINARY`
- `NMAP_DOCKER_IMAGE`
- `NMAP_TIMEOUT_SECONDS`

## 8) Basic Auth Endpoints

- `POST /api/v1/auth/register` creates a pentester account.
- `POST /api/v1/auth/login` returns JWT bearer token.
- `POST /api/v1/auth/refresh-token` rotates refresh token and returns new token pair.
- `POST /api/v1/auth/logout` revokes all refresh tokens for authenticated user.

## 9) Sample Tool Execute Request

`POST /api/v1/agent/execute`

```json
{
  "tool_intent": {
    "name": "nmap_scan",
    "params": {
      "target": "scanme.nmap.org",
      "top_ports": 100,
      "service_detection": false
    }
  }
}
```
