package handler

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"apant_be/internal/application/auth"
	appErrors "apant_be/internal/shared/errors"
	"apant_be/internal/shared/httpx"
)

type AuthHandler struct {
	service      *auth.Service
	cookieConfig AuthCookieConfig
}

type AuthCookieConfig struct {
	AccessName string
	RefreshName string
	CSRFName   string
	Domain     string
	Path       string
	SameSite   string
	Secure     bool
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

func NewAuthHandler(service *auth.Service, cookieConfig AuthCookieConfig) *AuthHandler {
	return &AuthHandler{
		service:      service,
		cookieConfig: normalizeCookieConfig(cookieConfig),
	}
}

func (h *AuthHandler) Register(c fiber.Ctx) error {
	var req auth.RegisterRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, fiber.StatusBadRequest, "invalid request body")
	}

	resp, err := h.service.Register(c.Context(), req)
	if err != nil {
		return writeAuthError(c, err)
	}

	if err := h.setAuthCookies(c, resp); err != nil {
		return httpx.JSONError(c, fiber.StatusInternalServerError, "failed to set auth cookies")
	}

	return httpx.JSONSuccess(c, http.StatusCreated, "user registered successfully", authPayload(resp))
}

func (h *AuthHandler) Login(c fiber.Ctx) error {
	var req auth.LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, fiber.StatusBadRequest, "invalid request body")
	}

	resp, err := h.service.Login(c.Context(), req)
	if err != nil {
		return writeAuthError(c, err)
	}

	if err := h.setAuthCookies(c, resp); err != nil {
		return httpx.JSONError(c, fiber.StatusInternalServerError, "failed to set auth cookies")
	}

	return httpx.JSONSuccess(c, http.StatusOK, "login successful", authPayload(resp))
}

func (h *AuthHandler) CSRF(c fiber.Ctx) error {
	csrfToken, err := generateCSRFToken()
	if err != nil {
		return httpx.JSONError(c, fiber.StatusInternalServerError, "failed to create csrf token")
	}

	h.setCSRFCookie(c, csrfToken)

	return httpx.JSONSuccess(c, http.StatusOK, "csrf ready", fiber.Map{})
}

func (h *AuthHandler) RefreshToken(c fiber.Ctx) error {
	var req auth.RefreshTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		req.RefreshToken = strings.TrimSpace(c.Cookies(h.cookieConfig.RefreshName))
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return httpx.JSONError(c, fiber.StatusBadRequest, "refresh_token is required")
	}

	resp, err := h.service.RefreshToken(c.Context(), req)
	if err != nil {
		return writeAuthError(c, err)
	}

	if err := h.setAuthCookies(c, resp); err != nil {
		return httpx.JSONError(c, fiber.StatusInternalServerError, "failed to set auth cookies")
	}

	return httpx.JSONSuccess(c, http.StatusOK, "token refreshed successfully", authPayload(resp))
}

func (h *AuthHandler) Logout(c fiber.Ctx) error {
	claims, ok := c.Locals("user").(jwt.MapClaims)
	if !ok {
		return httpx.JSONError(c, fiber.StatusUnauthorized, "invalid auth claims")
	}

	userID := strings.TrimSpace(fmt.Sprint(claims["sub"]))
	var req auth.LogoutRequest
	if len(c.Request().Body()) > 0 {
		if err := c.Bind().Body(&req); err != nil {
			return httpx.JSONError(c, fiber.StatusBadRequest, "invalid request body")
		}
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		req.RefreshToken = strings.TrimSpace(c.Cookies(h.cookieConfig.RefreshName))
	}

	if err := h.service.Logout(c.Context(), userID, req); err != nil {
		return writeAuthError(c, err)
	}

	h.clearAuthCookies(c)

	return httpx.JSONSuccess(c, http.StatusOK, "logout successful", fiber.Map{})
}

func authPayload(resp auth.AuthResponse) fiber.Map {
	return fiber.Map{
		"user":       resp.User,
		"expires_in": resp.ExpiresIn,
		"token_type": resp.TokenType,
	}
}

func (h *AuthHandler) setAuthCookies(c fiber.Ctx, resp auth.AuthResponse) error {
	csrfToken, err := generateCSRFToken()
	if err != nil {
		return err
	}

	sameSite := normalizeSameSite(h.cookieConfig.SameSite)
	accessMaxAge := resp.ExpiresIn
	if accessMaxAge <= 0 {
		accessMaxAge = int(h.cookieConfig.AccessTTL.Seconds())
	}
	refreshMaxAge := int(h.cookieConfig.RefreshTTL.Seconds())
	if refreshMaxAge <= 0 {
		refreshMaxAge = 7 * 24 * 3600
	}

	c.Cookie(&fiber.Cookie{
		Name:     h.cookieConfig.AccessName,
		Value:    resp.AccessToken,
		Domain:   strings.TrimSpace(h.cookieConfig.Domain),
		Path:     strings.TrimSpace(h.cookieConfig.Path),
		MaxAge:   accessMaxAge,
		HTTPOnly: true,
		Secure:   h.cookieConfig.Secure,
		SameSite: sameSite,
	})

	c.Cookie(&fiber.Cookie{
		Name:     h.cookieConfig.RefreshName,
		Value:    resp.RefreshToken,
		Domain:   strings.TrimSpace(h.cookieConfig.Domain),
		Path:     strings.TrimSpace(h.cookieConfig.Path),
		MaxAge:   refreshMaxAge,
		HTTPOnly: true,
		Secure:   h.cookieConfig.Secure,
		SameSite: sameSite,
	})

	h.setCSRFCookie(c, csrfToken)

	return nil
}

func (h *AuthHandler) clearAuthCookies(c fiber.Ctx) {
	sameSite := normalizeSameSite(h.cookieConfig.SameSite)
	clear := func(name string, httpOnly bool) {
		c.Cookie(&fiber.Cookie{
			Name:     name,
			Value:    "",
			Domain:   strings.TrimSpace(h.cookieConfig.Domain),
			Path:     strings.TrimSpace(h.cookieConfig.Path),
			MaxAge:   -1,
			HTTPOnly: httpOnly,
			Secure:   h.cookieConfig.Secure,
			SameSite: sameSite,
		})
	}

	clear(h.cookieConfig.AccessName, true)
	clear(h.cookieConfig.RefreshName, true)
	clear(h.cookieConfig.CSRFName, false)
}

func (h *AuthHandler) setCSRFCookie(c fiber.Ctx, token string) {
	sameSite := normalizeSameSite(h.cookieConfig.SameSite)
	refreshMaxAge := int(h.cookieConfig.RefreshTTL.Seconds())
	if refreshMaxAge <= 0 {
		refreshMaxAge = 7 * 24 * 3600
	}

	c.Cookie(&fiber.Cookie{
		Name:     h.cookieConfig.CSRFName,
		Value:    token,
		Domain:   strings.TrimSpace(h.cookieConfig.Domain),
		Path:     strings.TrimSpace(h.cookieConfig.Path),
		MaxAge:   refreshMaxAge,
		HTTPOnly: false,
		Secure:   h.cookieConfig.Secure,
		SameSite: sameSite,
	})
}

func normalizeSameSite(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return fiber.CookieSameSiteStrictMode
	case "none":
		return fiber.CookieSameSiteNoneMode
	default:
		return fiber.CookieSameSiteLaxMode
	}
}

func normalizeCookieConfig(cfg AuthCookieConfig) AuthCookieConfig {
	if strings.TrimSpace(cfg.AccessName) == "" {
		cfg.AccessName = "apant_access"
	}
	if strings.TrimSpace(cfg.RefreshName) == "" {
		cfg.RefreshName = "apant_refresh"
	}
	if strings.TrimSpace(cfg.CSRFName) == "" {
		cfg.CSRFName = "apant_csrf"
	}
	if strings.TrimSpace(cfg.Path) == "" {
		cfg.Path = "/"
	}
	if strings.TrimSpace(cfg.SameSite) == "" {
		cfg.SameSite = "lax"
	}
	if cfg.AccessTTL <= 0 {
		cfg.AccessTTL = 24 * time.Hour
	}
	if cfg.RefreshTTL <= 0 {
		cfg.RefreshTTL = 7 * 24 * time.Hour
	}
	return cfg
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func writeAuthError(c fiber.Ctx, err error) error {
	code, msg := appErrors.Resolve(err)
	return httpx.JSONError(c, code, msg)
}
