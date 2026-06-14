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

// PDFConverter renders an HTML document into PDF bytes.
type PDFConverter interface {
	HTMLToPDF(ctx context.Context, html []byte) ([]byte, error)
}
