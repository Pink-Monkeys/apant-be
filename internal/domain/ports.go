package domain

import "context"

type AIGenerateInput struct {
	Model  string
	System string
	User   string
	// Effort is an optional reasoning-effort hint ("low", "medium", "high") for
	// reasoning-capable models. Providers that support it (OpenAI Responses API
	// with a reasoning model) translate it to the provider's reasoning control;
	// everyone else ignores it. Empty means "let the provider decide" — the
	// pre-existing behavior. Cheap decision steps pass "low" to cut latency/cost;
	// synthesis steps pass "high" so report quality is preserved.
	Effort string
}

type AIGenerateOutput struct {
	Model string
	Text  string
	RawID string
	// Token usage as reported by the provider (all 0 when the provider does not
	// report a `usage` object). InputTokens/OutputTokens are the prompt/completion
	// split; TotalTokens is the provider's own total when present, else 0 (callers
	// treat 0 as "derive from Input+Output").
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// AIGateway abstracts provider-specific AI integration.
type AIGateway interface {
	Generate(ctx context.Context, provider string, in AIGenerateInput) (AIGenerateOutput, error)
	// Validate reports whether a (provider, model) selection is usable before a
	// scan does any expensive work: the provider must exist, be enabled, and have
	// a key; a non-empty model must exist and be enabled for that provider. It
	// returns a user-facing error (via shared/errors) when the selection is bad.
	//
	// On success it also returns the model's frozen price snapshot — the price in
	// effect at this moment — so the caller can store it on the scan record before
	// any work runs. The snapshot is unpriced (see FrozenPrice.Priced) when the
	// model has no price configured or when model is empty.
	Validate(ctx context.Context, provider, model string) (FrozenPrice, error)
}

type ToolIntent struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params"`
}

type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// ToolExecutor executes approved tool intents.
type ToolExecutor interface {
	Execute(intent *ToolIntent) map[string]any
}

// ToolPolicy validates tool intents before execution.
type ToolPolicy interface {
	Validate(intent *ToolIntent) error
}

// ToolRegistry exposes available tools for API discovery.
type ToolRegistry interface {
	List() []ToolInfo
}
