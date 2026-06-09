package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"apant_be/internal/application/pentest"
	"apant_be/internal/interfaces/http/middleware"
	appErrors "apant_be/internal/shared/errors"
	"apant_be/internal/shared/httpx"
)

type ScanHandler struct {
	service *pentest.Service
}

func NewScanHandler(service *pentest.Service) *ScanHandler {
	return &ScanHandler{service: service}
}

func (h *ScanHandler) Health(c fiber.Ctx) error {
	return httpx.JSONSuccess(c, http.StatusOK, "service is healthy", fiber.Map{"status": "ok"})
}

func (h *ScanHandler) Providers(c fiber.Ctx) error {
	return httpx.JSONSuccess(c, http.StatusOK, "providers retrieved successfully", fiber.Map{"providers": h.service.Providers()})
}

func (h *ScanHandler) Chat(c fiber.Ctx) error {
	var req pentest.ChatRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.service.Chat(c.Context(), req)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "chat response generated successfully", resp)
}

func (h *ScanHandler) AgentChat(c fiber.Ctx) error {
	var req pentest.AgentChatRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.service.AgentChat(c.Context(), req)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "agent chat response generated successfully", resp)
}

func (h *ScanHandler) AgentExecute(c fiber.Ctx) error {
	var req pentest.AgentExecuteRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.service.AgentExecute(c.Context(), req)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "agent execution completed successfully", resp)
}

func (h *ScanHandler) AgentLoop(c fiber.Ctx) error {
	var req pentest.AgentChatRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}

	if userID, ok := middleware.GetUserID(c); ok {
		req.UserID = userID
	}

	resp, err := h.service.AgentLoop(c.Context(), req)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "agent loop completed successfully", resp)
}

func (h *ScanHandler) ListScans(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	scans, err := h.service.ListScans(c.Context(), userID)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "scans retrieved successfully", scans)
}

func (h *ScanHandler) GetScan(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	scan, err := h.service.GetScan(c.Context(), c.Params("id"), userID)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "scan retrieved successfully", scan)
}

func (h *ScanHandler) Tools(c fiber.Ctx) error {
	return httpx.JSONSuccess(c, http.StatusOK, "tools retrieved successfully", fiber.Map{"tools": h.service.ListTools()})
}

func writeError(c fiber.Ctx, err error) error {
	code, msg := appErrors.Resolve(err)
	return httpx.JSONError(c, code, msg)
}
