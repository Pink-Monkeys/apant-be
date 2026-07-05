package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"apant_be/internal/domain"
	"apant_be/internal/shared/httpx"
)

// RequireRole gates a route to users whose JWT role claim matches one of the
// allowed roles. It must run after Protected, which populates the claims in
// context; the role is read from the token, so no DB lookup is needed.
func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[strings.ToLower(strings.TrimSpace(r))] = struct{}{}
	}

	return func(c fiber.Ctx) error {
		role, ok := GetUserRole(c)
		if !ok {
			return httpx.JSONError(c, fiber.StatusUnauthorized, "invalid auth claims")
		}
		if _, permitted := allowed[strings.ToLower(role)]; !permitted {
			return httpx.JSONError(c, fiber.StatusForbidden, "insufficient permissions")
		}
		return c.Next()
	}
}

// GetUserRole extracts the role claim set by Protected.
func GetUserRole(c fiber.Ctx) (string, bool) {
	claims, ok := c.Locals("user").(jwt.MapClaims)
	if !ok {
		return "", false
	}
	role, ok := claims["role"].(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(role), true
}

// IsAdmin reports whether the authenticated caller holds the admin role. Handlers
// use it to widen a query from "own records" to "all records" without a separate
// admin-only route.
func IsAdmin(c fiber.Ctx) bool {
	role, ok := GetUserRole(c)
	return ok && strings.EqualFold(strings.TrimSpace(role), domain.RoleAdmin)
}
