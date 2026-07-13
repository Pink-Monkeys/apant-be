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

type DashboardHandler struct {
	service *pentest.Service
}

func NewDashboardHandler(service *pentest.Service) *DashboardHandler {
	return &DashboardHandler{service: service}
}

func (h *DashboardHandler) RegisterRoutes(router fiber.Router) {
	router.Get("/dashboard/summary", h.GetSummary)
	router.Get("/dashboard/top-categories", h.GetTopCategories)
	router.Get("/dashboard/scan-ranking", h.GetScanRanking)
}

func (h *DashboardHandler) GetSummary(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}
	if middleware.IsAdmin(c) {
		userID = ""
	}

	rangeKey, from, to, err := parseDashboardRangeQuery(c)
	if err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, err.Error())
	}

	summary, err := h.service.DashboardSummary(c.Context(), userID, rangeKey, from, to)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "dashboard summary retrieved", summary)
}

func (h *DashboardHandler) GetTopCategories(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}
	if middleware.IsAdmin(c) {
		userID = ""
	}

	rangeKey, from, to, err := parseDashboardRangeQuery(c)
	if err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, err.Error())
	}

	limit := 5
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return httpx.JSONError(c, http.StatusBadRequest, "invalid limit")
		}
		limit = parsed
	}

	categories, err := h.service.TopCategories(c.Context(), userID, rangeKey, from, to, limit)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "top categories retrieved", categories)
}

func (h *DashboardHandler) GetScanRanking(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}
	if middleware.IsAdmin(c) {
		userID = ""
	}

	rangeKey, from, to, err := parseDashboardRangeQuery(c)
	if err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, err.Error())
	}

	ranking, err := h.service.ScanRanking(c.Context(), userID, rangeKey, from, to)
	if err != nil {
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "scan ranking retrieved", ranking)
}

// parseDashboardRangeQuery reads the shared range/from/to query params used by
// both dashboard endpoints. from/to are RFC3339 timestamps and take precedence
// over range when both are present (see resolveDateRange in the pentest
// service).
func parseDashboardRangeQuery(c fiber.Ctx) (rangeKey string, from, to time.Time, err error) {
	rangeKey = c.Query("range")

	if raw := c.Query("from"); raw != "" {
		from, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return "", time.Time{}, time.Time{}, fmt.Errorf("invalid from (use RFC3339, e.g. 2026-07-01T00:00:00Z)")
		}
	}
	if raw := c.Query("to"); raw != "" {
		to, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return "", time.Time{}, time.Time{}, fmt.Errorf("invalid to (use RFC3339, e.g. 2026-07-13T23:59:59Z)")
		}
	}

	return rangeKey, from, to, nil
}
