package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

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
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

type openAIRequest struct {
	Model string `json:"model"`
	Input []any  `json:"input"`
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

	b, err := json.Marshal(reqBody)
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/responses", bytes.NewReader(b))
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}

	if resp.StatusCode >= 300 {
		return domain.AIGenerateOutput{}, domain.NewProviderError(resp.StatusCode, body)
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

	return domain.AIGenerateOutput{Model: out.Model, Text: text, RawID: out.ID}, nil
}
