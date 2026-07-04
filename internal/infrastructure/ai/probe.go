package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"apant_be/internal/domain"
)

// ProbeResult reports the outcome of verifying a provider's credentials and,
// when supported, the models it exposes.
type ProbeResult struct {
	OK     bool
	Models []string
}

var probeClient = &http.Client{Timeout: 15 * time.Second}

// ProbeProvider verifies an API key against the provider and, for
// OpenAI-compatible providers, lists available models via GET /v1/models.
// Anthropic has no stable public list endpoint, so it does a minimal auth check
// and returns no model list (the admin enters models manually).
func ProbeProvider(ctx context.Context, adapterType, apiKey, baseURL string) (ProbeResult, error) {
	switch adapterType {
	case domain.AdapterOpenAICompatible, domain.AdapterOpenAIChat:
		// Both dialects expose the same GET /v1/models listing endpoint.
		return probeOpenAI(ctx, apiKey, baseURL)
	case domain.AdapterAnthropic:
		return probeAnthropic(ctx, apiKey)
	default:
		return ProbeResult{}, fmt.Errorf("unsupported adapter type: %q", adapterType)
	}
}

func probeOpenAI(ctx context.Context, apiKey, baseURL string) (ProbeResult, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = defaultOpenAIBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/models", nil)
	if err != nil {
		return ProbeResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := probeClient.Do(req)
	if err != nil {
		return ProbeResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ProbeResult{OK: false}, fmt.Errorf("authentication failed (%d)", resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return ProbeResult{OK: false}, fmt.Errorf("provider error (%d): %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		// Auth clearly succeeded (2xx); we just could not parse the list.
		return ProbeResult{OK: true}, nil
	}

	models := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			models = append(models, id)
		}
	}
	return ProbeResult{OK: true, Models: models}, nil
}

// probeAnthropic checks the key with a minimal models list request. Anthropic
// exposes GET /v1/models; if that ever changes, a 2xx still confirms auth.
func probeAnthropic(ctx context.Context, apiKey string) (ProbeResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	if err != nil {
		return ProbeResult{}, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := probeClient.Do(req)
	if err != nil {
		return ProbeResult{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ProbeResult{OK: false}, fmt.Errorf("authentication failed (%d)", resp.StatusCode)
	}
	if resp.StatusCode >= 300 {
		return ProbeResult{OK: false}, fmt.Errorf("provider error (%d): %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ProbeResult{OK: true}, nil
	}
	models := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			models = append(models, id)
		}
	}
	return ProbeResult{OK: true, Models: models}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
