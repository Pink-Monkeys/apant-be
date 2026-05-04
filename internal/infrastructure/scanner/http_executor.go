package scanner

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"apant_be/internal/domain"
)

type HTTPScannerConfig struct {
	BaseURL string
	Timeout time.Duration
}

type HTTPScannerExecutor struct {
	baseURL string
	client  *http.Client
}

func NewHTTPScannerExecutor(cfg HTTPScannerConfig) *HTTPScannerExecutor {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &HTTPScannerExecutor{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (e *HTTPScannerExecutor) Execute(intent *domain.ToolIntent) map[string]any {
	if intent == nil {
		return map[string]any{"status": "error", "error": "nil tool intent"}
	}

	name := strings.TrimSpace(strings.ToLower(intent.Name))
	if name != "nmap_scan" {
		return map[string]any{
			"status": "error",
			"tool":   name,
			"error":  "tool is not implemented by scanner service",
		}
	}

	payload, err := json.Marshal(intent)
	if err != nil {
		return map[string]any{"status": "error", "tool": name, "error": "failed to encode request"}
	}

	req, err := http.NewRequest(http.MethodPost, e.baseURL+"/execute", bytes.NewReader(payload))
	if err != nil {
		return map[string]any{"status": "error", "tool": name, "error": "failed to create request"}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return map[string]any{"status": "error", "tool": name, "error": "scanner service unavailable"}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return map[string]any{
			"status": "error",
			"tool":   name,
			"error":  "invalid response from scanner service",
			"raw":    strings.TrimSpace(string(body)),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out["status"] = "error"
		out["http_status"] = resp.StatusCode
		return out
	}

	return out
}
