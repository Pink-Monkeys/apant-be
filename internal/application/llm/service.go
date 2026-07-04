package llm

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"apant_be/internal/domain"
	appErrors "apant_be/internal/shared/errors"
)

// Cipher encrypts/decrypts provider API keys. Implemented by crypto.Cipher.
type Cipher interface {
	Encrypt(plaintext string) ([]byte, error)
	Decrypt(sealed []byte) (string, error)
}

// Prober verifies a provider's credentials and lists its models. Implemented by
// a thin adapter over ai.ProbeProvider.
type Prober interface {
	Probe(ctx context.Context, adapterType, apiKey, baseURL string) (ok bool, models []string, err error)
}

// CacheInvalidator lets the service evict a stale adapter from the AI gateway
// cache after a mutation. Implemented by *ai.DBGateway.
type CacheInvalidator interface {
	Invalidate(name string)
	InvalidateAll()
}

type Service struct {
	repo    domain.LLMProviderRepository
	cipher  Cipher
	prober  Prober
	gateway CacheInvalidator
}

func NewService(repo domain.LLMProviderRepository, cipher Cipher, prober Prober, gateway CacheInvalidator) *Service {
	return &Service{repo: repo, cipher: cipher, prober: prober, gateway: gateway}
}

func validAdapter(t string) bool {
	return t == domain.AdapterOpenAICompatible || t == domain.AdapterAnthropic
}

// --- Admin: providers ---

func (s *Service) ListProviders(ctx context.Context) ([]ProviderResponse, error) {
	providers, err := s.repo.ListProviders(ctx)
	if err != nil {
		return nil, appErrors.Wrap(http.StatusInternalServerError, "failed to list providers", err)
	}
	out := make([]ProviderResponse, 0, len(providers))
	for _, p := range providers {
		out = append(out, s.toProviderResponse(p))
	}
	return out, nil
}

func (s *Service) GetProvider(ctx context.Context, id string) (ProviderResponse, error) {
	p, found, err := s.repo.GetProvider(ctx, id)
	if err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load provider", err)
	}
	if !found {
		return ProviderResponse{}, appErrors.New(http.StatusNotFound, "provider not found")
	}
	return s.toProviderResponse(p), nil
}

func (s *Service) CreateProvider(ctx context.Context, req CreateProviderRequest) (ProviderResponse, error) {
	name := strings.TrimSpace(req.Name)
	adapter := strings.TrimSpace(req.AdapterType)
	if name == "" {
		return ProviderResponse{}, appErrors.New(http.StatusBadRequest, "name is required")
	}
	if !validAdapter(adapter) {
		return ProviderResponse{}, appErrors.New(http.StatusBadRequest, "adapter_type must be openai-compatible or anthropic")
	}

	enc, err := s.cipher.Encrypt(strings.TrimSpace(req.APIKey))
	if err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to secure api key", err)
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	p := domain.LLMProvider{
		ID:          uuid.NewString(),
		Name:        name,
		AdapterType: adapter,
		APIKeyEnc:   enc,
		BaseURL:     strings.TrimSpace(req.BaseURL),
		Enabled:     enabled,
	}
	if err := s.repo.CreateProvider(ctx, p); err != nil {
		return ProviderResponse{}, appErrors.New(http.StatusConflict, err.Error())
	}

	created, _, _ := s.repo.GetProvider(ctx, p.ID)
	return s.toProviderResponse(created), nil
}

func (s *Service) UpdateProvider(ctx context.Context, id string, req UpdateProviderRequest) (ProviderResponse, error) {
	existing, found, err := s.repo.GetProvider(ctx, id)
	if err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load provider", err)
	}
	if !found {
		return ProviderResponse{}, appErrors.New(http.StatusNotFound, "provider not found")
	}
	oldName := existing.Name

	if req.Name != nil {
		existing.Name = strings.TrimSpace(*req.Name)
	}
	if req.AdapterType != nil {
		adapter := strings.TrimSpace(*req.AdapterType)
		if !validAdapter(adapter) {
			return ProviderResponse{}, appErrors.New(http.StatusBadRequest, "adapter_type must be openai-compatible or anthropic")
		}
		existing.AdapterType = adapter
	}
	if req.BaseURL != nil {
		existing.BaseURL = strings.TrimSpace(*req.BaseURL)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	// APIKeyEnc==nil signals the repo to keep the stored key. Only re-encrypt
	// when a non-empty new key was supplied.
	existing.APIKeyEnc = nil
	if req.APIKey != nil && strings.TrimSpace(*req.APIKey) != "" {
		enc, encErr := s.cipher.Encrypt(strings.TrimSpace(*req.APIKey))
		if encErr != nil {
			return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to secure api key", encErr)
		}
		existing.APIKeyEnc = enc
	}

	if err := s.repo.UpdateProvider(ctx, existing); err != nil {
		return ProviderResponse{}, appErrors.New(http.StatusConflict, err.Error())
	}

	s.invalidate(oldName, existing.Name)

	updated, _, _ := s.repo.GetProvider(ctx, id)
	return s.toProviderResponse(updated), nil
}

func (s *Service) DeleteProvider(ctx context.Context, id string) error {
	existing, found, err := s.repo.GetProvider(ctx, id)
	if err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to load provider", err)
	}
	if !found {
		return appErrors.New(http.StatusNotFound, "provider not found")
	}
	if err := s.repo.DeleteProvider(ctx, id); err != nil {
		return appErrors.Wrap(http.StatusInternalServerError, "failed to delete provider", err)
	}
	s.invalidate(existing.Name)
	return nil
}

// --- Admin: models ---

func (s *Service) AddModel(ctx context.Context, providerID string, req AddModelRequest) (ProviderResponse, error) {
	provider, found, err := s.repo.GetProvider(ctx, providerID)
	if err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load provider", err)
	}
	if !found {
		return ProviderResponse{}, appErrors.New(http.StatusNotFound, "provider not found")
	}
	modelID := strings.TrimSpace(req.ModelID)
	if modelID == "" {
		return ProviderResponse{}, appErrors.New(http.StatusBadRequest, "model_id is required")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	m := domain.LLMModel{
		ID:         uuid.NewString(),
		ProviderID: providerID,
		ModelID:    modelID,
		Label:      strings.TrimSpace(req.Label),
		Enabled:    enabled,
	}
	if err := s.repo.AddModel(ctx, m); err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to add model", err)
	}
	s.invalidate(provider.Name)
	updated, _, _ := s.repo.GetProvider(ctx, providerID)
	return s.toProviderResponse(updated), nil
}

func (s *Service) UpdateModel(ctx context.Context, providerID, modelID string, req UpdateModelRequest) (ProviderResponse, error) {
	provider, found, err := s.repo.GetProvider(ctx, providerID)
	if err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load provider", err)
	}
	if !found {
		return ProviderResponse{}, appErrors.New(http.StatusNotFound, "provider not found")
	}

	var target domain.LLMModel
	ok := false
	for _, m := range provider.Models {
		if m.ID == modelID {
			target = m
			ok = true
			break
		}
	}
	if !ok {
		return ProviderResponse{}, appErrors.New(http.StatusNotFound, "model not found")
	}

	if req.ModelID != nil {
		target.ModelID = strings.TrimSpace(*req.ModelID)
	}
	if req.Label != nil {
		target.Label = strings.TrimSpace(*req.Label)
	}
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
	}
	if err := s.repo.UpdateModel(ctx, target); err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to update model", err)
	}
	s.invalidate(provider.Name)
	updated, _, _ := s.repo.GetProvider(ctx, providerID)
	return s.toProviderResponse(updated), nil
}

func (s *Service) DeleteModel(ctx context.Context, providerID, modelID string) (ProviderResponse, error) {
	provider, found, err := s.repo.GetProvider(ctx, providerID)
	if err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load provider", err)
	}
	if !found {
		return ProviderResponse{}, appErrors.New(http.StatusNotFound, "provider not found")
	}
	if err := s.repo.DeleteModel(ctx, modelID); err != nil {
		return ProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to delete model", err)
	}
	s.invalidate(provider.Name)
	updated, _, _ := s.repo.GetProvider(ctx, providerID)
	return s.toProviderResponse(updated), nil
}

// --- Admin: test / fetch ---

// TestProvider verifies the stored key (or an override supplied in the request)
// and returns the models the provider exposes, when available.
func (s *Service) TestProvider(ctx context.Context, id string) (TestProviderResponse, error) {
	p, found, err := s.repo.GetProvider(ctx, id)
	if err != nil {
		return TestProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load provider", err)
	}
	if !found {
		return TestProviderResponse{}, appErrors.New(http.StatusNotFound, "provider not found")
	}
	if len(p.APIKeyEnc) == 0 {
		return TestProviderResponse{}, appErrors.New(http.StatusBadRequest, "provider has no api key to test")
	}
	apiKey, err := s.cipher.Decrypt(p.APIKeyEnc)
	if err != nil {
		return TestProviderResponse{}, appErrors.Wrap(http.StatusInternalServerError, "failed to read api key", err)
	}

	ok, models, probeErr := s.prober.Probe(ctx, p.AdapterType, apiKey, p.BaseURL)
	if probeErr != nil {
		return TestProviderResponse{OK: false, Message: probeErr.Error()}, nil
	}
	return TestProviderResponse{OK: ok, Message: "provider reachable", AvailableModels: models}, nil
}

// --- Pentester: options ---

func (s *Service) Options(ctx context.Context) ([]OptionProvider, error) {
	providers, err := s.repo.ListProviders(ctx)
	if err != nil {
		return nil, appErrors.Wrap(http.StatusInternalServerError, "failed to list providers", err)
	}
	out := make([]OptionProvider, 0, len(providers))
	for _, p := range providers {
		if !p.Enabled || len(p.APIKeyEnc) == 0 {
			continue
		}
		models := make([]string, 0, len(p.Models))
		for _, m := range p.Models {
			if m.Enabled {
				models = append(models, m.ModelID)
			}
		}
		if len(models) == 0 {
			continue
		}
		out = append(out, OptionProvider{Name: p.Name, AdapterType: p.AdapterType, Models: models})
	}
	return out, nil
}

// --- helpers ---

func (s *Service) toProviderResponse(p domain.LLMProvider) ProviderResponse {
	resp := ProviderResponse{
		ID:          p.ID,
		Name:        p.Name,
		AdapterType: p.AdapterType,
		BaseURL:     p.BaseURL,
		Enabled:     p.Enabled,
		HasKey:      len(p.APIKeyEnc) > 0,
		Models:      make([]ModelResponse, 0, len(p.Models)),
	}
	if resp.HasKey {
		if plain, err := s.cipher.Decrypt(p.APIKeyEnc); err == nil {
			resp.KeyLast4 = last4(plain)
		}
	}
	for _, m := range p.Models {
		resp.Models = append(resp.Models, ModelResponse{
			ID:      m.ID,
			ModelID: m.ModelID,
			Label:   m.Label,
			Enabled: m.Enabled,
		})
	}
	return resp
}

func last4(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}

// invalidate evicts every affected provider name from the gateway cache. Called
// after any mutation so the next scan sees fresh config.
func (s *Service) invalidate(names ...string) {
	if s.gateway == nil {
		return
	}
	for _, n := range names {
		if strings.TrimSpace(n) != "" {
			s.gateway.Invalidate(n)
		}
	}
}
