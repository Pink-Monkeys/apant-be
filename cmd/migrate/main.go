package main

import (
	"flag"
	"log"
	"strings"

	"apant_be/internal/config"
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

	if err := db.ApplyMigrations(postgresDB.DB); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	log.Printf("migrations applied successfully")
}
