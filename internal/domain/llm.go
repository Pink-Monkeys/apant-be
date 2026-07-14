package domain

import (
	"context"
	"errors"
	"time"
)

// Adapter types map a provider row to a concrete AI code adapter. A provider's
// display name is free-form, but its behaviour is decided by AdapterType.
const (
	// AdapterOpenAICompatible targets OpenAI's Responses API (/v1/responses).
	// Use it for OpenAI itself.
	AdapterOpenAICompatible = "openai-compatible"
	// AdapterOpenAIChat targets the widely-implemented Chat Completions API
	// (/v1/chat/completions with a `messages` array). Use it for providers that
	// expose an OpenAI-compatible chat endpoint but not the Responses API:
	// DeepSeek, Gemini (OpenAI mode), OpenRouter, Groq, local servers, etc.
	AdapterOpenAIChat = "openai-chat"
	AdapterAnthropic  = "anthropic"
)

// ErrLLMProviderNotFound is returned when a provider id/name does not resolve.
var ErrLLMProviderNotFound = errors.New("llm provider not found")

// LLMProvider is an admin-managed AI backend: a display name, an adapter type
// that selects the wire format, an encrypted API key, and an optional base URL
// (used by OpenAI-compatible providers pointing at non-OpenAI endpoints).
type LLMProvider struct {
	ID          string
	Name        string
	AdapterType string
	// APIKeyEnc holds the AES-GCM ciphertext of the API key. It is never
	// serialized to API responses; handlers expose only presence + last4.
	APIKeyEnc []byte
	BaseURL   string
	Enabled   bool
	Models    []LLMModel
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LLMModel is one selectable model belonging to a provider. ModelID is the
// exact string sent to the provider API (e.g. "gpt-5.4"); Label is optional
// display text.
type LLMModel struct {
	ID         string
	ProviderID string
	ModelID    string
	Label      string
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// UserLLMPreference is a user's chosen provider+model for scans. It is stored
// per user (keyed by UserID) so each account keeps its own selection,
// independent of other users and portable across devices.
type UserLLMPreference struct {
	UserID    string
	Provider  string
	Model     string
	UpdatedAt time.Time
}

// UserLLMPreferenceRepository persists each user's LLM selection. There is at
// most one preference row per user; Upsert creates or replaces it.
type UserLLMPreferenceRepository interface {
	Get(ctx context.Context, userID string) (UserLLMPreference, bool, error)
	Upsert(ctx context.Context, pref UserLLMPreference) error
}

// LLMProviderRepository persists providers and their models. Model rows are
// owned by their provider (cascade delete).
type LLMProviderRepository interface {
	ListProviders(ctx context.Context) ([]LLMProvider, error)
	GetProvider(ctx context.Context, id string) (LLMProvider, bool, error)
	// GetProviderByName resolves a provider by its display name, used to map the
	// scan request's `provider` field to a stored provider.
	GetProviderByName(ctx context.Context, name string) (LLMProvider, bool, error)
	CreateProvider(ctx context.Context, p LLMProvider) error
	UpdateProvider(ctx context.Context, p LLMProvider) error
	DeleteProvider(ctx context.Context, id string) error

	AddModel(ctx context.Context, m LLMModel) error
	UpdateModel(ctx context.Context, m LLMModel) error
	DeleteModel(ctx context.Context, id string) error

	// CountProviders reports how many providers exist, used to make the
	// env → DB seed migration idempotent.
	CountProviders(ctx context.Context) (int64, error)
}
