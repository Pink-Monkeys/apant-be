package bootstrap

import (
	"fmt"
	"log"
	"strings"
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

func BuildApp(cfg config.Config) (*fiber.App, string) {
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

	userRepo := buildUserRepository(cfg)
	reportRepo := buildReportRepository(cfg)
	scanRepo := buildScanRepository(cfg)

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
	return app, addr
}

func buildUserRepository(cfg config.Config) domain.UserRepository {
	storage := strings.ToLower(strings.TrimSpace(cfg.AuthStorage))
	if storage == "postgres" {
		postgresDB, err := db.NewPostgres(cfg.PostgresDSN())
		if err != nil {
			log.Printf("auth storage postgres unavailable, falling back to memory: %v", err)
			return db.NewMemoryUserRepository()
		}

		repo, err := db.NewPostgresUserRepository(postgresDB)
		if err != nil {
			log.Printf("auth repository postgres initialization failed, falling back to memory: %v", err)
			return db.NewMemoryUserRepository()
		}

		log.Printf("auth storage backend: postgres")
		return repo
	}

	log.Printf("auth storage backend: memory")
	return db.NewMemoryUserRepository()
}

func buildScanRepository(cfg config.Config) domain.ScanRepository {
	storage := strings.ToLower(strings.TrimSpace(cfg.ScanStorage))
	if storage == "postgres" {
		postgresDB, err := db.NewPostgres(cfg.PostgresDSN())
		if err != nil {
			log.Printf("scan storage postgres unavailable, falling back to memory: %v", err)
			return db.NewMemoryScanRepository()
		}

		repo, err := db.NewScanRepositoryPostgres(postgresDB)
		if err != nil {
			log.Printf("scan repository postgres initialization failed, falling back to memory: %v", err)
			return db.NewMemoryScanRepository()
		}

		log.Printf("scan storage backend: postgres")
		return repo
	}

	log.Printf("scan storage backend: memory")
	return db.NewMemoryScanRepository()
}

func buildReportRepository(cfg config.Config) domain.ReportRepository {
	storage := strings.ToLower(strings.TrimSpace(cfg.ReportStorage))
	if storage == "postgres" {
		postgresDB, err := db.NewPostgres(cfg.PostgresDSN())
		if err != nil {
			log.Printf("report storage postgres unavailable, falling back to memory: %v", err)
			return db.NewMemoryReportRepository()
		}

		repo, err := db.NewPostgresReportRepository(postgresDB)
		if err != nil {
			log.Printf("report repository postgres initialization failed, falling back to memory: %v", err)
			return db.NewMemoryReportRepository()
		}

		log.Printf("report storage backend: postgres")
		return repo
	}

	log.Printf("report storage backend: memory")
	return db.NewMemoryReportRepository()
}
