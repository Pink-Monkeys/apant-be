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

This project uses GORM with `gormigrate` for schema migrations.
All migration steps are defined in Go (no raw SQL), and executed via a dedicated command.
The `migrations/` folder is not used in this workflow.

Run migrations up:

```powershell
go run .\cmd\migrate
```

Rollback last migration:

```powershell
go run .\cmd\migrate -rollback
```

Rollback to a specific migration ID:

```powershell
go run .\cmd\migrate -rollback-to 20260429_create_auth_tables
```

Migration rules:

- Add new migration entries in `internal/infrastructure/db/migrations.go`.
- Do not edit old migration IDs that already ran in shared environments.
- Keep rollback logic safe and reversible.
- Migrations are executed explicitly (not during app startup).

## 4) Docker Quick Start (API Only, External DB)

This section focuses only on running this Go Fiber API image.
PostgreSQL is assumed to be provided by another service/environment.

### 4.1 Build Image

```powershell
docker build -t apant-be:latest .
```

### 4.2 Prepare Runtime Env

Copy template:

```powershell
Copy-Item .env.example .env
```

Then set at least these values in `.env`:

```dotenv
APP_ENV=production
PORT=8000

AUTH_STORAGE=postgres
JWT_SECRET=replace-with-strong-secret

DB_HOST=your-postgres-host
DB_PORT=5432
DB_USER=your-postgres-user
DB_PASSWORD=your-postgres-password
DB_NAME=apant_be
DB_SSLMODE=disable
DB_TIMEZONE=Asia/Jakarta
```

Notes:

- Docker image packages the app binary, not your environment-specific secrets/config.
- `.env` (or `-e`) is still required to inject runtime config like DB host, credentials, JWT secret, and API keys.
- If a variable is not set, app uses default from `internal/config/config.go` when available.

### 4.3 Run Container

```powershell
docker run -d --name apant-be -p 8000:8000 --env-file .env apant-be:latest
```

If scanner endpoints need to run Docker commands from inside API container (high privilege, only for trusted environments):

```powershell
docker rm -f apant-be
docker run -d --name apant-be -p 8000:8000 --env-file .env -v //var/run/docker.sock:/var/run/docker.sock apant-be:latest
```

### 4.4 Verify

```powershell
docker logs -f apant-be
curl http://localhost:8000/api/v1/health
```

### 4.5 Stop

```powershell
docker rm -f apant-be
```

Optional remove image:

```powershell
docker rmi apant-be:latest
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
- `SCANNER_BASE_URL`
- `SCANNER_TIMEOUT_SECONDS`
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
