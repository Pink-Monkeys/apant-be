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
		AllowOrigins: cfg.AllowedOrigins,
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
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

	pentestService := pentest.NewService(aiGateway, executor, policy, registry, sessionRepo)
	authService := auth.NewService(
		userRepo,
		cfg.JWTSecret,
		time.Duration(cfg.AuthTokenTTLHours)*time.Hour,
		time.Duration(cfg.AuthRefreshTokenTTLHours)*time.Hour,
	)
	authHandler := handler.NewAuthHandler(authService)
	scanHandler := handler.NewScanHandler(pentestService)
	sessionHandler := handler.NewSessionHandler(pentestService)

	httpInterface.RegisterRouter(app, authHandler, scanHandler, sessionHandler, cfg.JWTSecret)

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
