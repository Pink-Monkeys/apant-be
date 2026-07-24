package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"apant_be/internal/domain"
	appErrors "apant_be/internal/shared/errors"
)

// Decryptor turns a stored ciphertext API key back into plaintext. Implemented
// by crypto.Cipher; kept as an interface so this package does not depend on the
// concrete cipher.
type Decryptor interface {
	Decrypt(sealed []byte) (string, error)
}

// DBGateway resolves providers from the database at request time: it looks up a
// provider by name (case-insensitive), decrypts its key, and builds the adapter
// via BuildProvider. Built adapters are cached and invalidated on CRUD so the
// key is not decrypted on every call.
//
// It satisfies domain.AIGateway, so existing callers of Generate(ctx, provider,
// in) work unchanged — only the meaning of `provider` shifts from a static name
// to a DB provider name.
type DBGateway struct {
	repo   domain.LLMProviderRepository
	cipher Decryptor
	mu     sync.RWMutex
	cache  map[string]cachedProvider // keyed by lowercased provider name
}

type cachedProvider struct {
	provider  Provider
	updatedAt int64 // provider row UpdatedAt unix nanos, to detect staleness
}

func NewDBGateway(repo domain.LLMProviderRepository, cipher Decryptor) *DBGateway {
	return &DBGateway{
		repo:   repo,
		cipher: cipher,
		cache:  make(map[string]cachedProvider),
	}
}

// Invalidate drops the cached adapter for a provider name so the next Generate
// rebuilds it from fresh DB state. Called by the admin service after CRUD.
func (g *DBGateway) Invalidate(name string) {
	key := strings.ToLower(strings.TrimSpace(name))
	g.mu.Lock()
	delete(g.cache, key)
	g.mu.Unlock()
}

// InvalidateAll clears the whole cache (e.g. after a bulk change).
func (g *DBGateway) InvalidateAll() {
	g.mu.Lock()
	g.cache = make(map[string]cachedProvider)
	g.mu.Unlock()
}

func (g *DBGateway) Generate(ctx context.Context, provider string, in domain.AIGenerateInput) (domain.AIGenerateOutput, error) {
	name := strings.TrimSpace(provider)
	if name == "" {
		return domain.AIGenerateOutput{}, fmt.Errorf("provider is required")
	}

	adapter, err := g.resolve(ctx, name)
	if err != nil {
		return domain.AIGenerateOutput{}, err
	}
	out, err := adapter.Generate(ctx, in)
	// Fold this call's token usage into the per-scan tally when one is installed
	// on the context (see domain.ContextWithTokenTally). Single chokepoint for
	// every caller of Generate; a no-op when no tally is present.
	if err == nil {
		domain.TokenTallyFromContext(ctx).Add(out.InputTokens, out.OutputTokens, out.TotalTokens)
	}
	return out, err
}

// Validate checks a (provider, model) selection without building an adapter or
// calling the provider. Used by scan entry points to fail fast on a bad choice
// before any expensive work (upload extraction, recon).
func (g *DBGateway) Validate(ctx context.Context, provider, model string) (domain.FrozenPrice, error) {
	name := strings.TrimSpace(provider)
	if name == "" {
		return domain.FrozenPrice{}, appErrors.New(http.StatusBadRequest, "provider is required")
	}

	p, found, err := g.repo.GetProviderByName(ctx, name)
	if err != nil {
		return domain.FrozenPrice{}, appErrors.Wrap(http.StatusInternalServerError, "failed to load provider", err)
	}
	if !found {
		return domain.FrozenPrice{}, appErrors.New(http.StatusBadRequest, fmt.Sprintf("unknown provider: %s", provider))
	}
	if !p.Enabled {
		return domain.FrozenPrice{}, appErrors.New(http.StatusBadRequest, fmt.Sprintf("provider %q is disabled", p.Name))
	}
	if len(p.APIKeyEnc) == 0 {
		return domain.FrozenPrice{}, appErrors.New(http.StatusBadRequest, fmt.Sprintf("provider %q has no API key configured", p.Name))
	}

	model = strings.TrimSpace(model)
	if model == "" {
		// No model pinned: nothing to price. Unpriced snapshot.
		return domain.FrozenPrice{}, nil
	}
	for _, m := range p.Models {
		if m.ModelID == model {
			if !m.Enabled {
				return domain.FrozenPrice{}, appErrors.New(http.StatusBadRequest, fmt.Sprintf("model %q is disabled for provider %q", model, p.Name))
			}
			// Freeze the model's current price at this moment.
			return m.FrozenPrice(), nil
		}
	}
	return domain.FrozenPrice{}, appErrors.New(http.StatusBadRequest, fmt.Sprintf("model %q is not available for provider %q", model, p.Name))
}

func (g *DBGateway) resolve(ctx context.Context, name string) (Provider, error) {
	p, found, err := g.repo.GetProviderByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("load provider %q: %w", name, err)
	}
	if !found {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	if !p.Enabled {
		return nil, fmt.Errorf("provider %q is disabled", p.Name)
	}
	if len(p.APIKeyEnc) == 0 {
		return nil, fmt.Errorf("provider %q has no API key configured", p.Name)
	}

	key := strings.ToLower(p.Name)
	stamp := p.UpdatedAt.UnixNano()

	// Fast path: cached adapter that matches the current row version.
	g.mu.RLock()
	if c, ok := g.cache[key]; ok && c.updatedAt == stamp {
		g.mu.RUnlock()
		return c.provider, nil
	}
	g.mu.RUnlock()

	apiKey, err := g.cipher.Decrypt(p.APIKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt key for provider %q: %w", p.Name, err)
	}

	adapter, err := BuildProvider(p.AdapterType, apiKey, p.BaseURL, "")
	if err != nil {
		return nil, err
	}

	g.mu.Lock()
	g.cache[key] = cachedProvider{provider: adapter, updatedAt: stamp}
	g.mu.Unlock()

	return adapter, nil
}
