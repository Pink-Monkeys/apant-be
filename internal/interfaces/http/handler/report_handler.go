package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

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
	router.Get("/reports/targets", h.ListReportTargets)
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

	// Admins may generate a report from any user's scan; the report is still
	// attributed to the scan's owner, not the admin. An empty userID skips the
	// ownership check in the service layer.
	if middleware.IsAdmin(c) {
		userID = ""
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

	// Admins see every user's reports; pentesters see only their own. An empty
	// userID filter means "all" in the repository layer.
	if middleware.IsAdmin(c) {
		userID = ""
	}

	req, err := parseListReportsRequest(c)
	if err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, err.Error())
	}

	result, err := h.service.ListReports(c.Context(), userID, req)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "reports retrieved", result)
}

// parseListReportsRequest reads range/from/to/target/page/limit query params.
// from/to are RFC3339 timestamps (e.g. 2026-07-01T00:00:00Z); range is a
// shorthand key resolved server-side (see resolveDateRange in the pentest
// service) and takes precedence only when from/to are both absent.
func parseListReportsRequest(c fiber.Ctx) (pentest.ListReportsRequest, error) {
	req := pentest.ListReportsRequest{
		Range:  c.Query("range"),
		Target: c.Query("target"),
	}

	if raw := c.Query("from"); raw != "" {
		from, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return pentest.ListReportsRequest{}, fmt.Errorf("invalid from (use RFC3339, e.g. 2026-07-01T00:00:00Z)")
		}
		req.From = from
	}
	if raw := c.Query("to"); raw != "" {
		to, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return pentest.ListReportsRequest{}, fmt.Errorf("invalid to (use RFC3339, e.g. 2026-07-13T23:59:59Z)")
		}
		req.To = to
	}
	if raw := c.Query("page"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil || page < 1 {
			return pentest.ListReportsRequest{}, fmt.Errorf("invalid page")
		}
		req.Page = page
	}
	if raw := c.Query("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 0 || limit > 100 {
			return pentest.ListReportsRequest{}, fmt.Errorf("invalid limit (must be 0-100; 0 means unlimited)")
		}
		if limit == 0 {
			// Explicit "no limit" request (e.g. reports summary cards need every
			// matching row, not one page) — distinct from the zero-value default,
			// which normalizeReportFilter defaults to 20. Use the sentinel -1 so
			// "unset" and "explicitly unlimited" don't collide on 0.
			limit = -1
		}
		req.Limit = limit
	}

	return req, nil
}

func (h *ReportHandler) ListReportTargets(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}
	if middleware.IsAdmin(c) {
		userID = ""
	}

	targets, err := h.service.ListReportTargets(c.Context(), userID)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "targets retrieved", targets)
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
	// Admins may view any report; pentesters only their own.
	if !middleware.IsAdmin(c) && report.UserID != "" && report.UserID != userID {
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
	// Admins may delete any report; pentesters only their own.
	if !middleware.IsAdmin(c) && report.UserID != "" && report.UserID != userID {
		return httpx.JSONError(c, http.StatusForbidden, "access denied")
	}

	if err := h.service.DeleteReport(c.Context(), id); err != nil {
		return httpx.JSONError(c, http.StatusInternalServerError, "failed to delete report")
	}

	return httpx.JSONSuccess(c, http.StatusOK, "report deleted", fiber.Map{})
}
