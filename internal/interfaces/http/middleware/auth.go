package middleware

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"apant_be/internal/shared/httpx"
)

func Protected(jwtSecret string, accessCookieName string) fiber.Handler {
	return func(c fiber.Ctx) error {
		cookieName := strings.TrimSpace(accessCookieName)
		if cookieName == "" {
			cookieName = "apant_access"
		}

		tokenStr := extractBearerToken(c)
		if tokenStr == "" {
			tokenStr = strings.TrimSpace(c.Cookies(cookieName))
		}
		if tokenStr == "" {
			return httpx.JSONError(c, fiber.StatusUnauthorized, "missing authorization token")
		}

		claims := jwt.MapClaims{}
		_, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil {
			return httpx.JSONError(c, fiber.StatusUnauthorized, "invalid or expired token")
		}

		c.Locals("user", claims)

		return c.Next()
	}
}

func extractBearerToken(c fiber.Ctx) string {
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

func GetUserID(c fiber.Ctx) (string, bool) {
	claims, ok := c.Locals("user").(jwt.MapClaims)
	if !ok {
		return "", false
	}

	userID := strings.TrimSpace(fmt.Sprint(claims["sub"]))
	if userID == "" {
		return "", false
	}

	return userID, true
}
