package http

import (
	"github.com/gofiber/fiber/v3"

	"apant_be/internal/interfaces/http/handler"
	"apant_be/internal/interfaces/http/middleware"
)

func RegisterRouter(
	app *fiber.App,
	auth *handler.AuthHandler,
	scan *handler.ScanHandler,
	session *handler.SessionHandler,
	jwtSecret string,
) {
	api := app.Group("/api/v1")

	api.Get("/health", scan.Health)
	api.Get("/providers", scan.Providers)
	api.Post("/auth/register", auth.Register)
	api.Post("/auth/login", auth.Login)
	api.Post("/auth/refresh-token", auth.RefreshToken)

	protected := api.Group("/", middleware.Protected(jwtSecret))
	protected.Post("/auth/logout", auth.Logout)
	protected.Post("/chat", scan.Chat)
	protected.Post("/agent/chat", scan.AgentChat)
	protected.Post("/agent/execute", scan.AgentExecute)
	protected.Post("/agent/loop", scan.AgentLoop)
	protected.Get("/tools", scan.Tools)

	protected.Post("/sessions", session.Create)
	protected.Get("/sessions", session.List)
	protected.Get("/sessions/:id", session.Get)
}
