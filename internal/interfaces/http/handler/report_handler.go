package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"apant_be/internal/application/pentest"
	"apant_be/internal/interfaces/http/middleware"
	"apant_be/internal/shared/httpx"
)

type ReportHandler struct {
	service *pentest.Service
}

func NewReportHandler(service *pentest.Service) *ReportHandler {
	return &ReportHandler{service: service}
}

func (h *ReportHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/reports", h.CreateReport)
	router.Get("/reports", h.ListReports)
	router.Get("/reports/:id", h.GetReport)
	router.Delete("/reports/:id", h.DeleteReport)
}

func (h *ReportHandler) CreateReport(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	var req pentest.CreateReportRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "invalid request body")
	}

	report, err := h.service.CreateReportFromScan(c.Context(), req.ScanID, userID)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusCreated, "report created", pentest.NewReportResponse(report))
}

func (h *ReportHandler) ListReports(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	reports, err := h.service.ListReports(c.Context(), userID)
	if err != nil {
		return httpx.JSONError(c, http.StatusInternalServerError, "failed to list reports")
	}

	return httpx.JSONSuccess(c, http.StatusOK, "reports retrieved", reports)
}

func (h *ReportHandler) GetReport(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	id := c.Params("id")
	report, err := h.service.GetReport(c.Context(), id)
	if err != nil {
		return httpx.JSONError(c, http.StatusNotFound, "report not found")
	}
	if report.UserID != "" && report.UserID != userID {
		return httpx.JSONError(c, http.StatusForbidden, "access denied")
	}

	return httpx.JSONSuccess(c, http.StatusOK, "report retrieved", pentest.NewReportResponse(report))
}

func (h *ReportHandler) DeleteReport(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	id := c.Params("id")
	report, err := h.service.GetReport(c.Context(), id)
	if err != nil {
		return httpx.JSONError(c, http.StatusNotFound, "report not found")
	}
	if report.UserID != "" && report.UserID != userID {
		return httpx.JSONError(c, http.StatusForbidden, "access denied")
	}

	if err := h.service.DeleteReport(c.Context(), id); err != nil {
		return httpx.JSONError(c, http.StatusInternalServerError, "failed to delete report")
	}

	return httpx.JSONSuccess(c, http.StatusOK, "report deleted", fiber.Map{})
}
