package domain

import "context"

type AIGenerateInput struct {
	Model  string
	System string
	User   string
}

type AIGenerateOutput struct {
	Model string
	Text  string
	RawID string
}

// AIGateway abstracts provider-specific AI integration.
type AIGateway interface {
	Generate(ctx context.Context, provider string, in AIGenerateInput) (AIGenerateOutput, error)
	// Validate reports whether a (provider, model) selection is usable before a
	// scan does any expensive work: the provider must exist, be enabled, and have
	// a key; a non-empty model must exist and be enabled for that provider. It
	// returns a user-facing error (via shared/errors) when the selection is bad.
	Validate(ctx context.Context, provider, model string) error
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
