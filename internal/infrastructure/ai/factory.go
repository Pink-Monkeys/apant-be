package ai

import (
	"fmt"

	"apant_be/internal/domain"
)

// BuildProvider constructs a concrete Provider from a stored provider's adapter
// type, decrypted API key, and base URL. It is the single place that maps a
// domain.LLMProvider adapter type to a wire-format adapter.
func BuildProvider(adapterType, apiKey, baseURL, model string) (Provider, error) {
	switch adapterType {
	case domain.AdapterOpenAICompatible:
		return NewOpenAIProviderWithBaseURL(apiKey, model, baseURL), nil
	case domain.AdapterAnthropic:
		return NewClaudeProvider(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unsupported adapter type: %q", adapterType)
	}
}
