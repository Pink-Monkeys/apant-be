package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

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

// StaticScan accepts a multipart upload of source code (a .zip archive) and runs
// a SAST analysis synchronously, returning the full result once complete —
// mirroring the dynamic /agent/loop flow.
func (h *ScanHandler) StaticScan(c fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return httpx.JSONError(c, http.StatusUnauthorized, "invalid auth claims")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return httpx.JSONError(c, http.StatusBadRequest, "a source archive must be uploaded in the 'file' field")
	}

	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".zip") {
		return httpx.JSONError(c, http.StatusBadRequest, "only .zip archives are supported")
	}

	// Persist the upload to a private temp file; the service takes ownership and
	// removes it after extraction.
	tmpPath := filepath.Join(os.TempDir(), "apant-upload-"+uuid.NewString()+".zip")
	if err := c.SaveFile(fileHeader, tmpPath); err != nil {
		return httpx.JSONError(c, http.StatusInternalServerError, "failed to store upload")
	}

	maxSteps := 0
	if v := strings.TrimSpace(c.FormValue("max_steps")); v != "" {
		maxSteps, _ = strconv.Atoi(v)
	}

	resp, err := h.service.StaticScan(c.Context(), pentest.StaticScanRequest{
		UserID:      userID,
		SessionID:   strings.TrimSpace(c.FormValue("session_id")),
		Provider:    c.FormValue("provider"),
		Model:       c.FormValue("model"),
		Description: c.FormValue("description"),
		MaxSteps:    maxSteps,
		SourceName:  fileHeader.Filename,
		ZipPath:     tmpPath,
	})
	if err != nil {
		// StaticScan removes the temp file and workspace on its own error paths.
		return writeError(c, err)
	}

	return httpx.JSONSuccess(c, http.StatusOK, "static scan completed", resp)
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

func (h *ScanHandler) ScanTypes(c fiber.Ctx) error {
	return httpx.JSONSuccess(c, http.StatusOK, "scan types retrieved successfully", fiber.Map{"scan_types": h.service.ScanTypes()})
}

func writeError(c fiber.Ctx, err error) error {
	code, msg := appErrors.Resolve(err)
	return httpx.JSONError(c, code, msg)
}
