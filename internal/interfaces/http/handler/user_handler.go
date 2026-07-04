package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"apant_be/internal/application/user"
	"apant_be/internal/domain"
	"apant_be/internal/interfaces/http/middleware"
	"apant_be/internal/shared/httpx"
)

type UserHandler struct {
	service *user.Service
}

func NewUserHandler(service *user.Service) *UserHandler {
	return &UserHandler{service: service}
}

// RegisterRoutes mounts admin-only user management on an already-protected
// group. Every route is gated by RequireRole(admin).
func (h *UserHandler) RegisterRoutes(router fiber.Router) {
	admin := middleware.RequireRole(domain.RoleAdmin)
	router.Get("/users", admin, h.List)
	router.Post("/users", admin, h.Create)
	router.Patch("/users/:id/role", admin, h.UpdateRole)
	router.Post("/users/:id/reset-password", admin, h.ResetPassword)
	router.Delete("/users/:id", admin, h.Delete)
}

func (h *UserHandler) Create(c fiber.Ctx) error {
	var req user.CreateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}

	created, err := h.service.Create(c.Context(), req)
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusCreated, "user created", created)
}

func (h *UserHandler) List(c fiber.Ctx) error {
	users, err := h.service.List(c.Context())
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "users retrieved", users)
}

func (h *UserHandler) UpdateRole(c fiber.Ctx) error {
	actingID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	var req user.UpdateRoleRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}

	updated, err := h.service.UpdateRole(c.Context(), actingID, c.Params("id"), req.Role)
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "role updated", updated)
}

func (h *UserHandler) ResetPassword(c fiber.Ctx) error {
	var req user.ResetPasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}

	if err := h.service.ResetPassword(c.Context(), c.Params("id"), req.NewPassword); err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "password reset", fiber.Map{})
}

func (h *UserHandler) Delete(c fiber.Ctx) error {
	actingID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	if err := h.service.Delete(c.Context(), actingID, c.Params("id")); err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "user deleted", fiber.Map{})
}
