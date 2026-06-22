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
	report *handler.ReportHandler,
	jwtSecret string,
	accessCookieName string,
	csrfCookieName string,
) {
	api := app.Group("/api/v1")

	api.Get("/health", scan.Health)
	api.Get("/providers", scan.Providers)
	api.Get("/auth/csrf", auth.CSRF)
	api.Post("/auth/register", auth.Register)
	api.Post("/auth/login", auth.Login)
	api.Post("/auth/refresh-token", middleware.CSRF(middleware.CSRFConfig{
		CookieName: csrfCookieName,
		HeaderName: "X-CSRF-Token",
	}), auth.RefreshToken)

	protected := api.Group("/", middleware.Protected(jwtSecret, accessCookieName), middleware.CSRF(middleware.CSRFConfig{
		CookieName: csrfCookieName,
		HeaderName: "X-CSRF-Token",
	}))
	protected.Post("/auth/logout", auth.Logout)
	protected.Get("/auth/me", auth.Me)
	protected.Patch("/auth/profile", auth.UpdateProfile)
	protected.Post("/auth/change-password", auth.ChangePassword)
	protected.Post("/chat", scan.Chat)
	protected.Post("/agent/chat", scan.AgentChat)
	protected.Post("/agent/execute", scan.AgentExecute)
	protected.Post("/agent/loop", scan.AgentLoop)
	protected.Post("/static/scan", scan.StaticScan)
	protected.Get("/tools", scan.Tools)
	protected.Get("/scan-types", scan.ScanTypes)

	protected.Post("/sessions", session.Create)
	protected.Get("/sessions", session.List)
	protected.Get("/sessions/:id", session.Get)

	protected.Get("/scans", scan.ListScans)
	protected.Get("/scans/:id", scan.GetScan)

	report.RegisterRoutes(protected)
}
