package llm

// ProviderResponse is the admin-facing view of a provider. It never carries the
// API key: only whether one is set and its last 4 characters.
type ProviderResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	AdapterType string          `json:"adapter_type"`
	BaseURL     string          `json:"base_url,omitempty"`
	Enabled     bool            `json:"enabled"`
	HasKey      bool            `json:"has_key"`
	KeyLast4    string          `json:"key_last4,omitempty"`
	Models      []ModelResponse `json:"models"`
}

type ModelResponse struct {
	ID      string `json:"id"`
	ModelID string `json:"model_id"`
	Label   string `json:"label,omitempty"`
	Enabled bool   `json:"enabled"`
	// Per-1M-token prices, or null when unpriced. A price of 0 is a genuine free
	// model (distinct from null). Present so the admin form can round-trip the
	// current values.
	PriceInPer1M  *float64 `json:"price_in_per_1m,omitempty"`
	PriceOutPer1M *float64 `json:"price_out_per_1m,omitempty"`
	Currency      *string  `json:"currency,omitempty"`
}

// OptionsResponse is the pentester-facing selector payload: enabled providers
// and their enabled models, with no key material at all.
type OptionProvider struct {
	Name        string   `json:"name"`
	AdapterType string   `json:"adapter_type"`
	Models      []string `json:"models"`
}

// SelectionResponse is a user's resolved provider+model choice for scans. It is
// always valid against the current catalog: a stored selection whose model was
// since removed/disabled is replaced with the first available option.
type SelectionResponse struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// SetSelectionRequest sets the caller's provider+model preference.
type SetSelectionRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type CreateProviderRequest struct {
	Name        string `json:"name"`
	AdapterType string `json:"adapter_type"`
	APIKey      string `json:"api_key"`
	BaseURL     string `json:"base_url"`
	Enabled     *bool  `json:"enabled"`
}

// UpdateProviderRequest patches a provider. A nil/omitted field leaves the
// stored value unchanged; an empty APIKey means "keep existing key".
type UpdateProviderRequest struct {
	Name        *string `json:"name"`
	AdapterType *string `json:"adapter_type"`
	APIKey      *string `json:"api_key"`
	BaseURL     *string `json:"base_url"`
	Enabled     *bool   `json:"enabled"`
}

type AddModelRequest struct {
	ModelID string `json:"model_id"`
	Label   string `json:"label"`
	Enabled *bool  `json:"enabled"`
	// Optional per-1M-token prices. Both must be supplied together (or neither).
	// Omitting both leaves the model unpriced; a value of 0 marks a free model.
	PriceInPer1M  *float64 `json:"price_in_per_1m"`
	PriceOutPer1M *float64 `json:"price_out_per_1m"`
	Currency      *string  `json:"currency"`
}

type UpdateModelRequest struct {
	ModelID *string `json:"model_id"`
	Label   *string `json:"label"`
	Enabled *bool   `json:"enabled"`
	// Price patch. Supplying both prices sets/overwrites them (0 => free). Omitting
	// them leaves the stored price unchanged. ClearPrice=true removes the price
	// (back to unpriced) and takes precedence over any supplied price.
	PriceInPer1M  *float64 `json:"price_in_per_1m"`
	PriceOutPer1M *float64 `json:"price_out_per_1m"`
	Currency      *string  `json:"currency"`
	ClearPrice    *bool    `json:"clear_price"`
}

// TestProviderResponse reports whether the stored key authenticates and, when
// the adapter supports listing, the models the provider offers.
type TestProviderResponse struct {
	OK              bool     `json:"ok"`
	Message         string   `json:"message"`
	AvailableModels []string `json:"available_models,omitempty"`
}
