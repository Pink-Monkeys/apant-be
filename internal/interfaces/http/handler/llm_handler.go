package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"apant_be/internal/application/llm"
	"apant_be/internal/domain"
	"apant_be/internal/interfaces/http/middleware"
	"apant_be/internal/shared/httpx"
)

type LLMHandler struct {
	service *llm.Service
}

func NewLLMHandler(service *llm.Service) *LLMHandler {
	return &LLMHandler{service: service}
}

// requireConfigured short-circuits with 503 when the LLM feature is not wired.
// The service is nil only in the pure-memory dev/test path (no encryption key),
// where provider management is unavailable; this guards every route from a nil
// dereference without repeating the check in each handler.
func (h *LLMHandler) requireConfigured(c fiber.Ctx) error {
	if h.service == nil {
		return httpx.JSONError(c, http.StatusServiceUnavailable, "llm provider management is not configured")
	}
	return c.Next()
}

// RegisterRoutes mounts the LLM routes on an already-protected group. Admin CRUD
// is gated by RequireRole(admin); the options selector is open to any
// authenticated user (pentesters select but cannot manage).
func (h *LLMHandler) RegisterRoutes(router fiber.Router) {
	router.Get("/llm/options", h.requireConfigured, h.Options)

	admin := middleware.RequireRole(domain.RoleAdmin)
	guard := h.requireConfigured
	router.Get("/llm/providers", guard, admin, h.ListProviders)
	router.Post("/llm/providers", guard, admin, h.CreateProvider)
	router.Get("/llm/providers/:id", guard, admin, h.GetProvider)
	router.Patch("/llm/providers/:id", guard, admin, h.UpdateProvider)
	router.Delete("/llm/providers/:id", guard, admin, h.DeleteProvider)
	router.Post("/llm/providers/:id/test", guard, admin, h.TestProvider)
	router.Post("/llm/providers/:id/models", guard, admin, h.AddModel)
	router.Patch("/llm/providers/:id/models/:modelId", guard, admin, h.UpdateModel)
	router.Delete("/llm/providers/:id/models/:modelId", guard, admin, h.DeleteModel)
}

func (h *LLMHandler) Options(c fiber.Ctx) error {
	options, err := h.service.Options(c.Context())
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "llm options retrieved", options)
}

func (h *LLMHandler) ListProviders(c fiber.Ctx) error {
	providers, err := h.service.ListProviders(c.Context())
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "providers retrieved", providers)
}

func (h *LLMHandler) GetProvider(c fiber.Ctx) error {
	provider, err := h.service.GetProvider(c.Context(), c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "provider retrieved", provider)
}

func (h *LLMHandler) CreateProvider(c fiber.Ctx) error {
	var req llm.CreateProviderRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}
	provider, err := h.service.CreateProvider(c.Context(), req)
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusCreated, "provider created", provider)
}

func (h *LLMHandler) UpdateProvider(c fiber.Ctx) error {
	var req llm.UpdateProviderRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}
	provider, err := h.service.UpdateProvider(c.Context(), c.Params("id"), req)
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "provider updated", provider)
}

func (h *LLMHandler) DeleteProvider(c fiber.Ctx) error {
	if err := h.service.DeleteProvider(c.Context(), c.Params("id")); err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "provider deleted", fiber.Map{})
}

func (h *LLMHandler) TestProvider(c fiber.Ctx) error {
	result, err := h.service.TestProvider(c.Context(), c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "provider tested", result)
}

func (h *LLMHandler) AddModel(c fiber.Ctx) error {
	var req llm.AddModelRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}
	provider, err := h.service.AddModel(c.Context(), c.Params("id"), req)
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusCreated, "model added", provider)
}

func (h *LLMHandler) UpdateModel(c fiber.Ctx) error {
	var req llm.UpdateModelRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}
	provider, err := h.service.UpdateModel(c.Context(), c.Params("id"), c.Params("modelId"), req)
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "model updated", provider)
}

func (h *LLMHandler) DeleteModel(c fiber.Ctx) error {
	provider, err := h.service.DeleteModel(c.Context(), c.Params("id"), c.Params("modelId"))
	if err != nil {
		return writeError(c, err)
	}
	return httpx.JSONSuccess(c, http.StatusOK, "model deleted", provider)
}
