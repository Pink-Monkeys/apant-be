package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v3"

	"apant_be/internal/shared/httpx"
)

type CSRFConfig struct {
	CookieName string
	HeaderName string
}

func CSRF(cfg CSRFConfig) fiber.Handler {
	cookieName := strings.TrimSpace(cfg.CookieName)
	if cookieName == "" {
		cookieName = "apant_csrf"
	}

	headerName := strings.TrimSpace(cfg.HeaderName)
	if headerName == "" {
		headerName = "X-CSRF-Token"
	}

	return func(c fiber.Ctx) error {
		method := strings.ToUpper(strings.TrimSpace(c.Method()))
		switch method {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}

		headerToken := strings.TrimSpace(c.Get(headerName))
		cookieToken := strings.TrimSpace(c.Cookies(cookieName))
		if headerToken == "" || cookieToken == "" {
			return httpx.JSONError(c, fiber.StatusForbidden, "csrf token missing")
		}

		if subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookieToken)) != 1 {
			return httpx.JSONError(c, fiber.StatusForbidden, "csrf token mismatch")
		}

		return c.Next()
	}
}
