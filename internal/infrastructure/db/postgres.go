package db

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Postgres struct {
	DSN string
	DB  *gorm.DB
}

func NewPostgres(dsn string) (*Postgres, error) {
	cleanDSN := strings.TrimSpace(dsn)
	if cleanDSN == "" {
		return nil, fmt.Errorf("postgres dsn is empty")
	}

	gormDB, err := gorm.Open(postgres.Open(cleanDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect postgres: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap sql db: %w", err)
	}

	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return &Postgres{DSN: cleanDSN, DB: gormDB}, nil
}
