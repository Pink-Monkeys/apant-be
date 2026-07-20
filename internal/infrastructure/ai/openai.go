package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"apant_be/internal/domain"
)

// defaultOpenAIBaseURL is the OpenAI API root. OpenAI-compatible providers
// (OpenRouter, Groq, local servers, …) override it via NewOpenAIProviderWithBaseURL.
const defaultOpenAIBaseURL = "https://api.openai.com"

type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return NewOpenAIProviderWithBaseURL(apiKey, model, "")
}

// NewOpenAIProviderWithBaseURL builds an OpenAI-compatible provider pointed at
// baseURL (e.g. "https://openrouter.ai/api"). An empty baseURL uses OpenAI.
func NewOpenAIProviderWithBaseURL(apiKey, model, baseURL string) *OpenAIProvider {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: providerHTTPTimeout()},
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

type openAIRequest struct {
	Model     string           `json:"model"`
	Input     []any            `json:"input"`
	Reasoning *openAIReasoning `json:"reasoning,omitempty"`
}

// openAIReasoning carries the reasoning-effort control for OpenAI reasoning
// models on the Responses API. Sent only when an effort is requested AND the
// model is reasoning-capable — the API rejects this field on non-reasoning
// models, so it must stay omitted otherwise (hence the pointer + omitempty).
type openAIReasoning struct {
	Effort string `json:"effort"`
}

// isReasoningModel reports whether a model name is an OpenAI reasoning model that
// accepts reasoning.effort (the o-series and gpt-5 family). A conservative name
// prefix check: matching too broadly would send an unsupported field to a plain
// chat model and hard-fail the request, so unknown names default to "no".
func isReasoningModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(m, "gpt-5"):
		return true
	case strings.HasPrefix(m, "o1"), strings.HasPrefix(m, "o3"), strings.HasPrefix(m, "o4"):
		return true
	default:
		return false
	}
}

// normalizeEffort validates a requested effort against the values the Responses
// API accepts, returning "" (omit the field) for anything unrecognized.
func normalizeEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "minimal":
		return "minimal"
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	default:
		return ""
	}
}

type openAIResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	// Responses API reports token usage as input/output/total (distinct from the
	// Chat Completions prompt/completion naming).
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func (p *OpenAIProvider) Generate(ctx context.Context, in domain.AIGenerateInput) (domain.AIGenerateOutput, error) {
	model := in.Model
	if model == "" {
		model = p.model
	}

	input := make([]any, 0, 2)

	if in.System != "" {
		input = append(input, map[string]any{
			"role": "system",
			"content": []map[string]string{
				{
					"type": "input_text",
					"text": in.System,
				},
			},
		})
	}

	input = append(input, map[string]any{
		"role": "user",
		"content": []map[string]string{
			{
				"type": "input_text",
				"text": in.User,
			},
		},
	})

	reqBody := openAIRequest{Model: model, Input: input}
	// Attach reasoning-effort only when requested AND the model supports it;
	// otherwise the field must be absent (see openAIReasoning).
	if effort := normalizeEffort(in.Effort); effort != "" && isReasoningModel(model) {
		reqBody.Reasoning = &openAIReasoning{Effort: effort}
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}

	status, body, err := doJSONWithRetry(ctx, p.client, "openai", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/responses", bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}

	if status >= 300 {
		return domain.AIGenerateOutput{}, domain.NewProviderError(status, body)
	}

	var out openAIResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return domain.AIGenerateOutput{}, err
	}

	var text string
	for _, item := range out.Output {
		for _, c := range item.Content {
			if c.Text != "" {
				text += c.Text
			}
		}
	}

	return domain.AIGenerateOutput{
		Model:        out.Model,
		Text:         text,
		RawID:        out.ID,
		InputTokens:  out.Usage.InputTokens,
		OutputTokens: out.Usage.OutputTokens,
		TotalTokens:  out.Usage.TotalTokens,
	}, nil
}
