# GORM Implementation Documentation

## 1) Overview

This project uses GORM as the ORM layer and `gormigrate` for schema migration management. The design goal is:

- No raw SQL in application logic.
- Explicit, deterministic migrations executed via a dedicated CLI.
- Clear separation between domain (interfaces) and infrastructure (concrete GORM adapters).

## 2) Architecture Placement

GORM is used only in the infrastructure layer to implement domain ports:

- `internal/infrastructure/db/postgres.go`
  - Creates and configures the GORM connection.
- `internal/infrastructure/db/user_repository_postgres.go`
  - Implements `domain.UserRepository` using GORM.
- `internal/infrastructure/db/migrations.go`
  - Defines and executes GORM migrations with `gormigrate`.

Domain and application layers do not import GORM, maintaining clean boundaries.

## 3) Connection Flow

### Entry

- `config.Load()` builds DB settings from environment.
- `config.Config.PostgresDSN()` returns a DSN string.
- `db.NewPostgres(dsn)` opens the GORM connection.

### Connection Setup

- Uses `gorm.Open(postgres.Open(dsn), &gorm.Config{ Logger: logger.Silent })`.
- Configures pool settings: max open conns, max idle conns, and connection lifetime.

## 4) Repository Flow (User + Refresh Token)

### Domain Port

- `domain.UserRepository` defines contract for auth persistence:
  - Create user
  - Find user
  - Create refresh token
  - Revoke refresh token

### Adapter Implementation

- `PostgresUserRepository` implements the above with GORM models:
  - `userModel` -> table `users`
  - `refreshTokenModel` -> table `refresh_tokens`

### Key Behaviors

- Username normalization: lowercase + trim before insert and query.
- Refresh tokens are stored by hash (not raw token).
- Revoke operations update `revoked_at` + `updated_at` fields.

## 5) Migration Flow (GORM + gormigrate)

### Migration Registry

- All migration steps live in `internal/infrastructure/db/migrations.go`.
- Each migration has:
  - `ID`: stable migration identifier.
  - `Migrate`: GORM migration (typically `AutoMigrate`).
  - `Rollback`: reverse operation with `Migrator().DropTable` or other safe rollback.
- Gormigrate uses table `gorm_migrations` to track applied versions.
- SQL migration files under `migrations/` are not used in this workflow.

### Running Migrations

CLI entry point: `cmd/migrate/main.go`

- Apply all pending migrations:

```powershell
go run .\cmd\migrate
```

- Rollback last migration:

```powershell
go run .\cmd\migrate -rollback
```

- Rollback to a specific migration ID:

```powershell
go run .\cmd\migrate -rollback-to 20260429_create_auth_tables
```

### Why Explicit Migrations

- Avoids unintended schema drift at runtime.
- Safer for production deployments.
- Keeps schema history in code and reviewable.

## 6) Data Flow: Auth + GORM

### Register Flow

1. HTTP handler parses request.
2. `auth.Service.Register` validates and hashes password.
3. `UserRepository.Create` persists user via GORM.
4. Refresh token hash stored via `CreateRefreshToken`.
5. Handler returns JSON response.

### Refresh Token Flow

1. Token hash lookup via `FindRefreshTokenByHash`.
2. Validity checks (revoked, expired).
3. Revoke old token and create new one.

### Logout Flow

1. `RevokeUserRefreshTokens` updates all active tokens for a user.
2. Optional revoke by token ID if provided.

## 7) Security Considerations

- Passwords are stored as bcrypt hash (never raw).
- Refresh tokens stored as hash (SHA256), raw token never persisted.
- GORM uses prepared statements under the hood, reducing SQL injection risk.
- Migration is explicit; no schema changes on app startup.

## 8) Best Practices Used

- Clean separation: ORM only in infrastructure adapters.
- Explicit migrations with version control via gormigrate.
- Transaction-safe migration operations.
- AutoMigrate used only in migration flow, not in production runtime.
- No raw SQL in application logic.

## 9) Notes and Gaps

- `ScanRepositoryPostgres` is not implemented yet.
- Session repository is still in-memory.
- Consider adding integration tests for repository methods.

## 10) How to Add a New Migration

1. Add a new `gormigrate.Migration` entry in `migrations()`.
2. Use `AutoMigrate` or explicit `Migrator()` calls.
3. Provide a safe rollback.
4. Run `go run .\cmd\migrate` locally and in CI/CD pipeline.

## 11) Step-by-Step: Changing Table Structure

Use the steps below whenever you add, change, or remove a column/table. This is the recommended development workflow for this codebase.

### Step 1: Add a New Migration Entry

Open `internal/infrastructure/db/migrations.go` and append a new migration with a new ID.

### Step 2: Implement the Schema Change

#### A) Add a new column

Use a lightweight patch struct so you do not accidentally alter unrelated columns.

```go
{
  ID: "20260429_add_email_to_users",
  Migrate: func(tx *gorm.DB) error {
    type userEmailPatch struct {
      Email string `gorm:"column:email;type:text"`
    }
    return tx.Table("users").AutoMigrate(&userEmailPatch{})
  },
  Rollback: func(tx *gorm.DB) error {
    return tx.Migrator().DropColumn("users", "email")
  },
},
```

#### B) Add a new table

Define a new model struct and run `AutoMigrate`.

```go
{
  ID: "20260429_create_scan_tables",
  Migrate: func(tx *gorm.DB) error {
    return tx.AutoMigrate(&scanModel{}, &scanResultModel{})
  },
  Rollback: func(tx *gorm.DB) error {
    return tx.Migrator().DropTable(&scanResultModel{}, &scanModel{})
  },
},
```

#### C) Rename a column safely

```go
{
  ID: "20260429_rename_username_to_handle",
  Migrate: func(tx *gorm.DB) error {
    return tx.Migrator().RenameColumn("users", "username", "handle")
  },
  Rollback: func(tx *gorm.DB) error {
    return tx.Migrator().RenameColumn("users", "handle", "username")
  },
},
```

#### D) Drop a column

```go
{
  ID: "20260429_drop_unused_column",
  Migrate: func(tx *gorm.DB) error {
    return tx.Migrator().DropColumn("users", "unused_column")
  },
  Rollback: func(tx *gorm.DB) error {
    return tx.Migrator().AddColumn(&userModel{}, "unused_column")
  },
},
```

### Step 3: Run the Migration Locally

```powershell
go run .\cmd\migrate
```

### Step 4: Validate Schema

- Ensure the app still compiles and tests pass.
- Verify the table structure in PostgreSQL.

### Step 5: Commit and Deploy

- Commit the migration changes.
- CI/CD will apply migrations using the same command when you deploy.

### Recommended Safety Workflow for Breaking Changes

If you need to remove or change a field that is already used by production code:

1. Add a new column (nullable) in a migration.
2. Update application code to write both old and new columns.
3. Backfill existing data if required.
4. Update application code to read only the new column.
5. Remove the old column in a later migration.
