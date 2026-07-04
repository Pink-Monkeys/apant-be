package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName                  string
	Port                     string
	AppEnv                   string
	OpenAIAPIKey             string
	OpenAIModel              string
	AnthropicKey             string
	AnthropicModel           string
	LLMEncryptionKey         string
	AllowedOrigins           []string
	JWTSecret                   string
	AuthAccessTokenTTLMinutes   int
	AuthRefreshTokenTTLHours    int
	AuthAbsoluteSessionTTLHours int
	AuthStorage              string
	ReportStorage            string
	ScanStorage              string
	DBHost                   string
	DBPort                   int
	AuthAccessCookieName     string
	AuthRefreshCookieName    string
	AuthCSRFCookieName       string
	AuthCookieDomain         string
	AuthCookiePath           string
	AuthCookieSameSite       string
	AuthCookieSecure         bool
	DBUser                   string
	DBPassword               string
	DBName                   string
	DBSSLMode                string
	DBTimeZone               string
	ScannerBaseURL           string
	ScannerTimeoutSeconds    int
	WorkspaceDir             string
	StaticMaxUploadBytes     int64
	StaticMaxFiles           int
	StaticMaxTotalBytes      int64
}

func Load() Config {
	_ = godotenv.Load()

	// STORAGE is the single source of truth for the persistence backend. The
	// per-component vars (AUTH/SCAN/REPORT_STORAGE) default to it so they cannot
	// silently diverge — set STORAGE=memory only for tests/dev-without-db.
	storage := strings.ToLower(strings.TrimSpace(getEnv("STORAGE", "postgres")))

	return Config{
		AppName:                  getEnv("APP_NAME", "apant_be"),
		Port:                     getEnv("PORT", "8000"),
		AppEnv:                   getEnv("APP_ENV", "development"),
		OpenAIAPIKey:             os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:              getEnv("OPENAI_MODEL", "gpt-5.4"),
		AnthropicKey:             os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:           getEnv("ANTHROPIC_MODEL", "claude-sonnet-4-5"),
		LLMEncryptionKey:         os.Getenv("LLM_ENCRYPTION_KEY"),
		AllowedOrigins:           splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173")),
		JWTSecret:                getEnv("JWT_SECRET", "change-me"),
		AuthAccessTokenTTLMinutes:   getEnvInt("AUTH_ACCESS_TOKEN_TTL_MINUTES", 15),
		AuthRefreshTokenTTLHours:    getEnvInt("AUTH_REFRESH_TOKEN_TTL_HOURS", 168),
		AuthAbsoluteSessionTTLHours: getEnvInt("AUTH_ABSOLUTE_SESSION_TTL_HOURS", 720),
		AuthStorage:              strings.ToLower(strings.TrimSpace(getEnv("AUTH_STORAGE", storage))),
		ReportStorage:            strings.ToLower(strings.TrimSpace(getEnv("REPORT_STORAGE", storage))),
		ScanStorage:              strings.ToLower(strings.TrimSpace(getEnv("SCAN_STORAGE", storage))),
		DBHost:                   getEnv("DB_HOST", "localhost"),
		DBPort:                   getEnvInt("DB_PORT", 5432),
		AuthAccessCookieName:     getEnv("AUTH_ACCESS_COOKIE_NAME", "apant_access"),
		AuthRefreshCookieName:    getEnv("AUTH_REFRESH_COOKIE_NAME", "apant_refresh"),
		AuthCSRFCookieName:       getEnv("AUTH_CSRF_COOKIE_NAME", "apant_csrf"),
		AuthCookieDomain:         getEnv("AUTH_COOKIE_DOMAIN", ""),
		AuthCookiePath:           getEnv("AUTH_COOKIE_PATH", "/"),
		AuthCookieSameSite:       getEnv("AUTH_COOKIE_SAMESITE", "lax"),
		AuthCookieSecure:         getEnvBool("AUTH_COOKIE_SECURE", false),
		DBUser:                   getEnv("DB_USER", "postgres"),
		DBPassword:               os.Getenv("DB_PASSWORD"),
		DBName:                   getEnv("DB_NAME", "apant_be"),
		DBSSLMode:                getEnv("DB_SSLMODE", "disable"),
		DBTimeZone:               getEnv("DB_TIMEZONE", "Asia/Jakarta"),
		ScannerBaseURL:           getEnv("SCANNER_BASE_URL", "http://localhost:8081"),
		ScannerTimeoutSeconds:    getEnvInt("SCANNER_TIMEOUT_SECONDS", 600),
		WorkspaceDir:             getEnv("WORKSPACE_DIR", "/workspace"),
		StaticMaxUploadBytes:     getEnvInt64("STATIC_MAX_UPLOAD_BYTES", 50*1024*1024),
		StaticMaxFiles:           getEnvInt("STATIC_MAX_FILES", 20000),
		StaticMaxTotalBytes:      getEnvInt64("STATIC_MAX_TOTAL_BYTES", 300*1024*1024),
	}
}

func (c Config) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		strings.TrimSpace(c.DBHost),
		strings.TrimSpace(c.DBUser),
		c.DBPassword,
		strings.TrimSpace(c.DBName),
		c.DBPort,
		strings.TrimSpace(c.DBSSLMode),
		strings.TrimSpace(c.DBTimeZone),
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}

	return parsed
}
