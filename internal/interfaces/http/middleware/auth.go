package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"apant_be/internal/shared/httpx"
)

func Protected(jwtSecret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := strings.TrimSpace(c.Get("Authorization"))
		if authHeader == "" {
			return httpx.JSONError(c, fiber.StatusUnauthorized, "missing authorization header")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return httpx.JSONError(c, fiber.StatusUnauthorized, "invalid authorization format")
		}

		tokenStr := strings.TrimSpace(parts[1])
		if tokenStr == "" {
			return httpx.JSONError(c, fiber.StatusUnauthorized, "empty bearer token")
		}

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
			if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return httpx.JSONError(c, fiber.StatusUnauthorized, "invalid or expired token")
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Locals("user", claims)
		}

		return c.Next()
	}
}
