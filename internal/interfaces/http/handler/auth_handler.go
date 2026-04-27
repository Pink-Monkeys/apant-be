package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"apant_be/internal/application/auth"
	appErrors "apant_be/internal/shared/errors"
	"apant_be/internal/shared/httpx"
)

type AuthHandler struct {
	service *auth.Service
}

func NewAuthHandler(service *auth.Service) *AuthHandler {
	return &AuthHandler{
		service: service,
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

	return httpx.JSONSuccess(c, http.StatusCreated, "user registered successfully", resp)
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

	return httpx.JSONSuccess(c, http.StatusOK, "login successful", resp)
}

func (h *AuthHandler) RefreshToken(c fiber.Ctx) error {
	var req auth.RefreshTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, fiber.StatusBadRequest, "invalid request body")
	}

	resp, err := h.service.RefreshToken(c.Context(), req)
	if err != nil {
		return writeAuthError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "token refreshed successfully", resp)
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

	if err := h.service.Logout(c.Context(), userID, req); err != nil {
		return writeAuthError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "logout successful", fiber.Map{})
}

func writeAuthError(c fiber.Ctx, err error) error {
	code, msg := appErrors.Resolve(err)
	return httpx.JSONError(c, code, msg)
}
