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
	AllowedOrigins           []string
	JWTSecret                string
	AuthTokenTTLHours        int
	AuthRefreshTokenTTLHours int
	AuthStorage              string
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
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		AppName:                  getEnv("APP_NAME", "apant_be"),
		Port:                     getEnv("PORT", "8000"),
		AppEnv:                   getEnv("APP_ENV", "development"),
		OpenAIAPIKey:             os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:              getEnv("OPENAI_MODEL", "gpt-5.4-mini"),
		AnthropicKey:             os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:           getEnv("ANTHROPIC_MODEL", "claude-sonnet-4-5"),
		AllowedOrigins:           splitCSV(getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000")),
		JWTSecret:                getEnv("JWT_SECRET", "change-me"),
		AuthTokenTTLHours:        getEnvInt("AUTH_TOKEN_TTL_HOURS", 24),
		AuthRefreshTokenTTLHours: getEnvInt("AUTH_REFRESH_TOKEN_TTL_HOURS", 168),
		AuthStorage:              strings.ToLower(getEnv("AUTH_STORAGE", "memory")),
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
		ScannerTimeoutSeconds:    getEnvInt("SCANNER_TIMEOUT_SECONDS", 60),
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

