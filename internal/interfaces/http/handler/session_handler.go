package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"apant_be/internal/application/pentest"
)

type SessionHandler struct {
	service *pentest.Service
}

func NewSessionHandler(service *pentest.Service) *SessionHandler {
	return &SessionHandler{service: service}
}

func (h *SessionHandler) Create(c fiber.Ctx) error {
	sess := h.service.CreateSession()
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    sess,
	})
}

func (h *SessionHandler) List(c fiber.Ctx) error {
	sessions := h.service.ListSessions()
	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"sessions": sessions,
		},
	})
}

func (h *SessionHandler) Get(c fiber.Ctx) error {
	sess, err := h.service.GetSession(c.Params("id"))
	if err != nil {
		return writeError(c, err)
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    sess,
	})
}
