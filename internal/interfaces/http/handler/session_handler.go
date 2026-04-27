package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"apant_be/internal/application/pentest"
	"apant_be/internal/shared/httpx"
)

type SessionHandler struct {
	service *pentest.Service
}

func NewSessionHandler(service *pentest.Service) *SessionHandler {
	return &SessionHandler{service: service}
}

func (h *SessionHandler) Create(c fiber.Ctx) error {
	sess := h.service.CreateSession()
	return httpx.JSONSuccess(c, http.StatusCreated, "session created successfully", sess)
}

func (h *SessionHandler) List(c fiber.Ctx) error {
	sessions := h.service.ListSessions()
	return httpx.JSONSuccess(c, http.StatusOK, "sessions retrieved successfully", fiber.Map{
		"sessions": sessions,
	})
}

func (h *SessionHandler) Get(c fiber.Ctx) error {
	sess, err := h.service.GetSession(c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "session retrieved successfully", sess)
}
