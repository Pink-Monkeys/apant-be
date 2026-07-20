package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"apant_be/internal/domain"
)

// OpenAIChatProvider talks to the Chat Completions API
// (POST {baseURL}/v1/chat/completions with a `messages` array). This is the
// widely-implemented OpenAI dialect, so it serves providers that are
// OpenAI-compatible at the chat endpoint but do not offer the Responses API:
// DeepSeek, Gemini (OpenAI mode), OpenRouter, Groq, local servers, etc.
//
// It is distinct from OpenAIProvider, which targets OpenAI's newer
// /v1/responses endpoint and is used for OpenAI itself.
type OpenAIChatProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIChatProvider builds a chat-completions provider. baseURL is required
// for non-OpenAI providers (e.g. "https://api.deepseek.com"); an empty baseURL
// falls back to OpenAI's host so the same adapter can also drive OpenAI's chat
// endpoint if ever needed.
func NewOpenAIChatProvider(apiKey, model, baseURL string) *OpenAIChatProvider {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAIChatProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{Timeout: providerHTTPTimeout()},
	}
}

func (p *OpenAIChatProvider) Name() string {
	return "openai-chat"
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	// Chat Completions reports token usage here (OpenAI, DeepSeek, OpenRouter,
	// Groq, … all follow this shape). Absent on some local servers → zero values.
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (p *OpenAIChatProvider) Generate(ctx context.Context, in domain.AIGenerateInput) (domain.AIGenerateOutput, error) {
	model := in.Model
	if model == "" {
		model = p.model
	}

	messages := make([]chatMessage, 0, 2)
	if in.System != "" {
		messages = append(messages, chatMessage{Role: "system", Content: in.System})
	}
	messages = append(messages, chatMessage{Role: "user", Content: in.User})

	reqBody := chatCompletionRequest{Model: model, Messages: messages}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}

	status, body, err := doJSONWithRetry(ctx, p.client, "openai-chat", func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	})
	if err != nil {
		log.Printf("openai-chat: request to %s failed: %v", p.baseURL, err)
		return domain.AIGenerateOutput{}, err
	}

	if status >= 300 {
		log.Printf("openai-chat: %s/v1/chat/completions model=%q status=%d body=%s",
			p.baseURL, model, status, truncate(string(body), 500))
		return domain.AIGenerateOutput{}, domain.NewProviderError(status, body)
	}

	var out chatCompletionResponse
	if err := json.Unmarshal(body, &out); err != nil {
		log.Printf("openai-chat: parse response failed: %v; raw=%s", err, truncate(string(body), 500))
		return domain.AIGenerateOutput{}, err
	}

	var text string
	for _, c := range out.Choices {
		if c.Message.Content != "" {
			text += c.Message.Content
		}
	}

	// An empty completion (e.g. model returned only reasoning tokens, or a
	// provider quirk) would otherwise surface as a confusing downstream failure.
	if text == "" {
		log.Printf("openai-chat: empty completion from %s model=%q; raw=%s",
			p.baseURL, model, truncate(string(body), 500))
	}

	return domain.AIGenerateOutput{
		Model:        out.Model,
		Text:         text,
		RawID:        out.ID,
		InputTokens:  out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
		TotalTokens:  out.Usage.TotalTokens,
	}, nil
}
