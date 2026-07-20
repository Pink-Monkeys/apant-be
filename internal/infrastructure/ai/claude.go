package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"apant_be/internal/domain"
)

type ClaudeProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewClaudeProvider(apiKey, model string) *ClaudeProvider {
	return &ClaudeProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: providerHTTPTimeout()},
	}
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

type claudeRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    string              `json:"system,omitempty"`
	Messages  []claudeMessageItem `json:"messages"`
}

type claudeMessageItem struct {
	Role    string              `json:"role"`
	Content []claudeContentItem `json:"content"`
}

type claudeContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	// Anthropic reports input/output tokens (no total — derived downstream).
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (p *ClaudeProvider) Generate(ctx context.Context, in domain.AIGenerateInput) (domain.AIGenerateOutput, error) {
	model := in.Model
	if model == "" {
		model = p.model
	}

	reqBody := claudeRequest{
		Model:     model,
		MaxTokens: 1500,
		System:    in.System,
		Messages: []claudeMessageItem{
			{
				Role: "user",
				Content: []claudeContentItem{
					{Type: "text", Text: in.User},
				},
			},
		},
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}

	status, body, err := doJSONWithRetry(ctx, p.client, "claude", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("content-type", "application/json")
		return req, nil
	})
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}

	if status >= 300 {
		return domain.AIGenerateOutput{}, domain.NewProviderError(status, body)
	}

	var out claudeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return domain.AIGenerateOutput{}, err
	}

	var text string
	for _, c := range out.Content {
		if c.Text != "" {
			text += c.Text
		}
	}

	return domain.AIGenerateOutput{
		Model:        out.Model,
		Text:         text,
		RawID:        out.ID,
		InputTokens:  out.Usage.InputTokens,
		OutputTokens: out.Usage.OutputTokens,
	}, nil
}
