package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ProviderError carries an upstream LLM provider failure with a clean,
// human-readable message and the provider's HTTP status, so the API layer can
// surface a sensible response instead of dumping raw JSON as "Bad Gateway".
// It lives in domain so both the AI adapters (which produce it) and the
// application services (which map it to an HTTP status) can reference it without
// a layering violation.
type ProviderError struct {
	// StatusCode is the HTTP status the provider returned (e.g. 429, 401).
	StatusCode int
	// Message is a short human-readable reason extracted from the provider's
	// error payload; falls back to the raw body when nothing better is found.
	Message string
}

func (e *ProviderError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("provider error (%d)", e.StatusCode)
	}
	return e.Message
}

// NewProviderError builds a ProviderError from a non-2xx response body, pulling
// the most useful human-readable string out of the common OpenAI/OpenRouter
// error shapes: {"error":{"message":...,"metadata":{"raw":...}}}.
func NewProviderError(statusCode int, body []byte) *ProviderError {
	msg := extractProviderMessage(body)
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	return &ProviderError{StatusCode: statusCode, Message: msg}
}

func extractProviderMessage(body []byte) string {
	var parsed struct {
		Error struct {
			Message  string `json:"message"`
			Metadata struct {
				Raw string `json:"raw"`
			} `json:"metadata"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	// metadata.raw (OpenRouter) is usually the most specific, human-facing text
	// (e.g. "<model> is temporarily rate-limited upstream. Please retry shortly").
	if raw := strings.TrimSpace(parsed.Error.Metadata.Raw); raw != "" {
		return raw
	}
	return strings.TrimSpace(parsed.Error.Message)
}
