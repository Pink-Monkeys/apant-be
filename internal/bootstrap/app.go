package bootstrap

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"apant_be/internal/application/auth"
	"apant_be/internal/application/pentest"
	"apant_be/internal/config"
	"apant_be/internal/domain"
	"apant_be/internal/infrastructure/ai"
	"apant_be/internal/infrastructure/db"
	"apant_be/internal/infrastructure/scanner"
	httpInterface "apant_be/internal/interfaces/http"
	"apant_be/internal/interfaces/http/handler"
)

func BuildApp(cfg config.Config) (*fiber.App, string, error) {
	app := fiber.New(fiber.Config{
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	}))

	openAIProvider := ai.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	claudeProvider := ai.NewClaudeProvider(cfg.AnthropicKey, cfg.AnthropicModel)
	aiGateway := ai.NewGateway(openAIProvider, claudeProvider)

	executor := scanner.NewHTTPScannerExecutor(scanner.HTTPScannerConfig{
		BaseURL: cfg.ScannerBaseURL,
		Timeout: time.Duration(cfg.ScannerTimeoutSeconds) * time.Second,
	})
	policy := scanner.NewToolPolicy()
	registry := scanner.NewRegistry()
	sessionRepo := db.NewMemorySessionRepository()

	// One shared Postgres connection for every repository that needs it.
	var postgresDB *db.Postgres
	if needsPostgres(cfg) {
		pg, err := db.NewPostgres(cfg.PostgresDSN())
		if err != nil {
			return nil, "", fmt.Errorf("connect postgres: %w", err)
		}
		postgresDB = pg
	}

	userRepo, err := buildUserRepository(cfg, postgresDB)
	if err != nil {
		return nil, "", err
	}
	reportRepo, err := buildReportRepository(cfg, postgresDB)
	if err != nil {
		return nil, "", err
	}
	scanRepo, err := buildScanRepository(cfg, postgresDB)
	if err != nil {
		return nil, "", err
	}

	pentestService := pentest.NewService(aiGateway, executor, policy, registry, sessionRepo, scanRepo, reportRepo)
	authService := auth.NewService(
		userRepo,
		cfg.JWTSecret,
		time.Duration(cfg.AuthTokenTTLHours)*time.Hour,
		time.Duration(cfg.AuthRefreshTokenTTLHours)*time.Hour,
	)
	authHandler := handler.NewAuthHandler(authService, handler.AuthCookieConfig{
		AccessName:  cfg.AuthAccessCookieName,
		RefreshName: cfg.AuthRefreshCookieName,
		CSRFName:    cfg.AuthCSRFCookieName,
		Domain:      cfg.AuthCookieDomain,
		Path:        cfg.AuthCookiePath,
		SameSite:    cfg.AuthCookieSameSite,
		Secure:      cfg.AuthCookieSecure,
		AccessTTL:   time.Duration(cfg.AuthTokenTTLHours) * time.Hour,
		RefreshTTL:  time.Duration(cfg.AuthRefreshTokenTTLHours) * time.Hour,
	})
	scanHandler := handler.NewScanHandler(pentestService)
	sessionHandler := handler.NewSessionHandler(pentestService)
	reportHandler := handler.NewReportHandler(pentestService)

	httpInterface.RegisterRouter(app, authHandler, scanHandler, sessionHandler, reportHandler, cfg.JWTSecret, cfg.AuthAccessCookieName, cfg.AuthCSRFCookieName)

	addr := fmt.Sprintf(":%s", cfg.Port)
	return app, addr, nil
}

func needsPostgres(cfg config.Config) bool {
	return cfg.AuthStorage == "postgres" ||
		cfg.ScanStorage == "postgres" ||
		cfg.ReportStorage == "postgres"
}

// Storage builders fail fast: when a backend is set to "postgres" but Postgres is
// unavailable, the app refuses to start instead of silently degrading to memory
// (which is what made scan/report data appear "saved" yet absent from the DB).

func buildUserRepository(cfg config.Config, pg *db.Postgres) (domain.UserRepository, error) {
	if cfg.AuthStorage == "postgres" {
		repo, err := db.NewPostgresUserRepository(pg)
		if err != nil {
			return nil, fmt.Errorf("auth repository (postgres): %w", err)
		}
		log.Printf("auth storage backend: postgres")
		return repo, nil
	}

	log.Printf("auth storage backend: memory")
	return db.NewMemoryUserRepository(), nil
}

func buildScanRepository(cfg config.Config, pg *db.Postgres) (domain.ScanRepository, error) {
	if cfg.ScanStorage == "postgres" {
		repo, err := db.NewScanRepositoryPostgres(pg)
		if err != nil {
			return nil, fmt.Errorf("scan repository (postgres): %w", err)
		}
		log.Printf("scan storage backend: postgres")
		return repo, nil
	}

	log.Printf("scan storage backend: memory")
	return db.NewMemoryScanRepository(), nil
}

func buildReportRepository(cfg config.Config, pg *db.Postgres) (domain.ReportRepository, error) {
	if cfg.ReportStorage == "postgres" {
		repo, err := db.NewPostgresReportRepository(pg)
		if err != nil {
			return nil, fmt.Errorf("report repository (postgres): %w", err)
		}
		log.Printf("report storage backend: postgres")
		return repo, nil
	}

	log.Printf("report storage backend: memory")
	return db.NewMemoryReportRepository(), nil
}
