package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"apant_be/internal/config"
	"apant_be/internal/infrastructure/crypto"
	"apant_be/internal/infrastructure/db"
)

func main() {
	rollback := flag.Bool("rollback", false, "rollback the last migration")
	rollbackTo := flag.String("rollback-to", "", "rollback to a specific migration ID")
	flag.Parse()

	cfg := config.Load()

	postgresDB, err := db.NewPostgres(cfg.PostgresDSN())
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}

	if *rollbackTo != "" {
		migrationID := strings.TrimSpace(*rollbackTo)
		if migrationID == "" {
			log.Fatal("rollback-to cannot be empty")
		}
		if err := db.RollbackToMigration(postgresDB.DB, migrationID); err != nil {
			log.Fatalf("migration rollback-to failed: %v", err)
		}
		log.Printf("rollback to migration %s completed", migrationID)
		return
	}

	if *rollback {
		if err := db.RollbackLastMigration(postgresDB.DB); err != nil {
			log.Fatalf("migration rollback failed: %v", err)
		}
		log.Printf("rollback last migration completed")
		return
	}

	if err := db.ApplyMigrations(postgresDB.DB, buildSeedConfig(cfg)); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	log.Printf("migrations applied successfully")
}

// buildSeedConfig assembles the data-seed inputs: an encrypt function (nil when
// no valid LLM_ENCRYPTION_KEY, which disables the provider seed) and a
// bootstrap admin (default creds unless overridden via ADMIN_BOOTSTRAP_* env).
func buildSeedConfig(cfg config.Config) db.SeedConfig {
	seed := db.SeedConfig{
		OpenAIAPIKey:   cfg.OpenAIAPIKey,
		OpenAIModel:    cfg.OpenAIModel,
		AnthropicKey:   cfg.AnthropicKey,
		AnthropicModel: cfg.AnthropicModel,
	}

	if cipher, err := crypto.NewCipher(cfg.LLMEncryptionKey); err != nil {
		log.Printf("LLM provider seed skipped: %v", err)
	} else {
		seed.EncryptKey = cipher.Encrypt
	}

	// Default admin credentials; override with ADMIN_BOOTSTRAP_* env vars.
	adminUser := getEnvDefault("ADMIN_BOOTSTRAP_USERNAME", "admin")
	adminEmail := getEnvDefault("ADMIN_BOOTSTRAP_EMAIL", "admin@apant.local")
	adminPass := getEnvDefault("ADMIN_BOOTSTRAP_PASSWORD", "ChangeMe123!")
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("admin seed skipped: hash password: %v", err)
		return seed
	}
	seed.AdminUsername = adminUser
	seed.AdminEmail = adminEmail
	seed.AdminPasswordHash = string(hash)
	return seed
}

func getEnvDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
