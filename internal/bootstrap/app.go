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
	"apant_be/internal/application/llm"
	"apant_be/internal/application/pentest"
	"apant_be/internal/application/user"
	"apant_be/internal/config"
	"apant_be/internal/domain"
	"apant_be/internal/infrastructure/ai"
	"apant_be/internal/infrastructure/crypto"
	"apant_be/internal/infrastructure/db"
	"apant_be/internal/infrastructure/scanner"
	httpInterface "apant_be/internal/interfaces/http"
	"apant_be/internal/interfaces/http/handler"
)

func BuildApp(cfg config.Config) (*fiber.App, string, error) {
	// BodyLimit must accommodate the largest allowed source upload (plus a small
	// multipart overhead) or fiber rejects the request before the handler runs.
	bodyLimit := int(cfg.StaticMaxUploadBytes) + 1*1024*1024

	// Timeouts are generous: static (SAST) and dynamic (agent loop) scans run
	// synchronously and can take a couple of minutes, and the SAST upload can be
	// tens of MB. ReadTimeout covers reading the upload; WriteTimeout covers the
	// long-running scan before the response is written.
	app := fiber.New(fiber.Config{
		ReadTimeout:  300 * time.Second,
		WriteTimeout: 600 * time.Second,
		BodyLimit:    bodyLimit,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-CSRF-Token"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	}))

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

	// LLM provider management: the cipher protects API keys at rest and is
	// required once anything is persisted to postgres. Built here so Phase 3/4
	// (dynamic providers + admin CRUD) can consume them.
	llmCipher, err := buildLLMCipher(cfg)
	if err != nil {
		return nil, "", err
	}
	llmRepo, err := buildLLMRepository(cfg, postgresDB)
	if err != nil {
		return nil, "", err
	}

	// The AI gateway resolves providers/models from the DB at request time,
	// decrypting each provider's key on demand. It requires a cipher, which is
	// always present when postgres is used (buildLLMCipher fails fast otherwise).
	aiGateway, dbGateway := buildAIGateway(cfg, llmRepo, llmCipher)

	// Admin LLM management service. It needs a cipher and cache invalidator; both
	// are only present in the DB-backed path (dbGateway != nil).
	var llmService *llm.Service
	if dbGateway != nil {
		llmService = llm.NewService(llmRepo, llmCipher, ai.NewProbeAdapter(), dbGateway)
	}
	llmHandler := handler.NewLLMHandler(llmService)

	pentestService := pentest.NewService(aiGateway, executor, policy, registry, sessionRepo, scanRepo, reportRepo)
	pentestService.ConfigureStatic(pentest.StaticConfig{
		WorkspaceDir:  cfg.WorkspaceDir,
		MaxFiles:      cfg.StaticMaxFiles,
		MaxTotalBytes: cfg.StaticMaxTotalBytes,
	})
	authService := auth.NewService(
		userRepo,
		cfg.JWTSecret,
		time.Duration(cfg.AuthAccessTokenTTLMinutes)*time.Minute,
		time.Duration(cfg.AuthRefreshTokenTTLHours)*time.Hour,
		time.Duration(cfg.AuthAbsoluteSessionTTLHours)*time.Hour,
	)
	authHandler := handler.NewAuthHandler(authService, handler.AuthCookieConfig{
		AccessName:  cfg.AuthAccessCookieName,
		RefreshName: cfg.AuthRefreshCookieName,
		CSRFName:    cfg.AuthCSRFCookieName,
		Domain:      cfg.AuthCookieDomain,
		Path:        cfg.AuthCookiePath,
		SameSite:    cfg.AuthCookieSameSite,
		Secure:      cfg.AuthCookieSecure,
		AccessTTL:   time.Duration(cfg.AuthAccessTokenTTLMinutes) * time.Minute,
		RefreshTTL:  time.Duration(cfg.AuthRefreshTokenTTLHours) * time.Hour,
	})
	scanHandler := handler.NewScanHandler(pentestService)
	sessionHandler := handler.NewSessionHandler(pentestService)
	reportHandler := handler.NewReportHandler(pentestService)

	userService := user.NewService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	httpInterface.RegisterRouter(app, authHandler, scanHandler, sessionHandler, reportHandler, llmHandler, userHandler, cfg.JWTSecret, cfg.AuthAccessCookieName, cfg.AuthCSRFCookieName)

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

// buildLLMCipher constructs the AES-GCM cipher for LLM API keys. It is required
// whenever a postgres-backed component runs (keys are persisted encrypted); in
// pure-memory mode it is optional and returns nil when unset.
func buildLLMCipher(cfg config.Config) (*crypto.Cipher, error) {
	if strings.TrimSpace(cfg.LLMEncryptionKey) == "" {
		if needsPostgres(cfg) {
			return nil, fmt.Errorf("LLM_ENCRYPTION_KEY is required when STORAGE=postgres")
		}
		return nil, nil
	}
	cipher, err := crypto.NewCipher(cfg.LLMEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("llm encryption key: %w", err)
	}
	return cipher, nil
}

// buildAIGateway wires the DB-backed gateway when a cipher is available
// (i.e. postgres, or memory with LLM_ENCRYPTION_KEY set). Without a cipher —
// pure-memory dev/test with no key — it falls back to a static gateway seeded
// from the legacy env keys so those flows keep working unchanged.
//
// The second return is the DB gateway (nil in the static fallback) so the LLM
// admin service can invalidate its cache after CRUD.
func buildAIGateway(cfg config.Config, repo domain.LLMProviderRepository, cipher *crypto.Cipher) (domain.AIGateway, *ai.DBGateway) {
	if cipher != nil {
		g := ai.NewDBGateway(repo, cipher)
		return g, g
	}

	log.Printf("ai gateway: static (no encryption key; using env-configured providers)")
	openAIProvider := ai.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	claudeProvider := ai.NewClaudeProvider(cfg.AnthropicKey, cfg.AnthropicModel)
	return ai.NewGateway(openAIProvider, claudeProvider), nil
}

func buildLLMRepository(cfg config.Config, pg *db.Postgres) (domain.LLMProviderRepository, error) {
	if cfg.ScanStorage == "postgres" || cfg.AuthStorage == "postgres" {
		repo, err := db.NewLLMRepositoryPostgres(pg)
		if err != nil {
			return nil, fmt.Errorf("llm repository (postgres): %w", err)
		}
		log.Printf("llm storage backend: postgres")
		return repo, nil
	}

	log.Printf("llm storage backend: memory")
	return db.NewMemoryLLMRepository(), nil
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
