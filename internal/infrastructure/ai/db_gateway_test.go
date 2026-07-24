package ai

import (
	"context"
	"testing"

	"apant_be/internal/domain"
)

// fakeRepo is a minimal LLMProviderRepository for gateway tests: only the
// lookups the gateway uses are meaningful.
type fakeRepo struct {
	provider domain.LLMProvider
	found    bool
	calls    int
}

func (r *fakeRepo) GetProviderByName(_ context.Context, _ string) (domain.LLMProvider, bool, error) {
	r.calls++
	return r.provider, r.found, nil
}

func (r *fakeRepo) ListProviders(context.Context) ([]domain.LLMProvider, error) { return nil, nil }
func (r *fakeRepo) GetProvider(context.Context, string) (domain.LLMProvider, bool, error) {
	return domain.LLMProvider{}, false, nil
}
func (r *fakeRepo) CreateProvider(context.Context, domain.LLMProvider) error { return nil }
func (r *fakeRepo) UpdateProvider(context.Context, domain.LLMProvider) error { return nil }
func (r *fakeRepo) DeleteProvider(context.Context, string) error             { return nil }
func (r *fakeRepo) AddModel(context.Context, domain.LLMModel) error          { return nil }
func (r *fakeRepo) UpdateModel(context.Context, domain.LLMModel) error       { return nil }
func (r *fakeRepo) DeleteModel(context.Context, string) error                { return nil }
func (r *fakeRepo) CountProviders(context.Context) (int64, error)            { return 0, nil }

// fakeCipher treats the ciphertext as the plaintext bytes, so tests avoid
// depending on the crypto package.
type fakeCipher struct{}

func (fakeCipher) Decrypt(sealed []byte) (string, error) { return string(sealed), nil }

func TestDBGatewayResolveErrors(t *testing.T) {
	t.Run("unknown provider", func(t *testing.T) {
		g := NewDBGateway(&fakeRepo{found: false}, fakeCipher{})
		if _, err := g.resolve(context.Background(), "nope"); err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})

	t.Run("disabled provider", func(t *testing.T) {
		g := NewDBGateway(&fakeRepo{found: true, provider: domain.LLMProvider{
			Name: "OpenAI", AdapterType: domain.AdapterOpenAICompatible,
			APIKeyEnc: []byte("k"), Enabled: false,
		}}, fakeCipher{})
		if _, err := g.resolve(context.Background(), "openai"); err == nil {
			t.Fatal("expected error for disabled provider")
		}
	})

	t.Run("no key", func(t *testing.T) {
		g := NewDBGateway(&fakeRepo{found: true, provider: domain.LLMProvider{
			Name: "OpenAI", AdapterType: domain.AdapterOpenAICompatible, Enabled: true,
		}}, fakeCipher{})
		if _, err := g.resolve(context.Background(), "openai"); err == nil {
			t.Fatal("expected error for provider without key")
		}
	})

	t.Run("unsupported adapter", func(t *testing.T) {
		g := NewDBGateway(&fakeRepo{found: true, provider: domain.LLMProvider{
			Name: "Weird", AdapterType: "mystery", APIKeyEnc: []byte("k"), Enabled: true,
		}}, fakeCipher{})
		if _, err := g.resolve(context.Background(), "weird"); err == nil {
			t.Fatal("expected error for unsupported adapter type")
		}
	})
}

func TestDBGatewayCaseInsensitiveAndCaches(t *testing.T) {
	repo := &fakeRepo{found: true, provider: domain.LLMProvider{
		Name: "OpenAI", AdapterType: domain.AdapterOpenAICompatible,
		APIKeyEnc: []byte("secret"), Enabled: true,
	}}
	g := NewDBGateway(repo, fakeCipher{})

	// Lowercased request name still resolves the "OpenAI" provider.
	a, err := g.resolve(context.Background(), "openai")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if a.Name() != "openai" {
		t.Fatalf("expected openai adapter, got %s", a.Name())
	}

	// Second call with same row version must hit the cache (adapter identity
	// preserved), even though the repo is queried for freshness each time.
	b, err := g.resolve(context.Background(), "OpenAI")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if a != b {
		t.Fatal("expected cached adapter to be reused for unchanged provider")
	}
}

func TestDBGatewayValidate(t *testing.T) {
	priceIn, priceOut := 3.0, 15.0
	base := domain.LLMProvider{
		Name: "OpenAI", AdapterType: domain.AdapterOpenAICompatible,
		APIKeyEnc: []byte("secret"), Enabled: true,
		Models: []domain.LLMModel{
			// Priced model: Validate must freeze this price.
			{ModelID: "gpt-5.4", Enabled: true, PriceInPer1M: &priceIn, PriceOutPer1M: &priceOut},
			// Enabled but no price configured: Validate must return an unpriced snapshot.
			{ModelID: "gpt-noprice", Enabled: true},
			{ModelID: "gpt-old", Enabled: false},
		},
	}

	newGW := func(p domain.LLMProvider, found bool) *DBGateway {
		return NewDBGateway(&fakeRepo{provider: p, found: found}, fakeCipher{})
	}

	t.Run("empty provider", func(t *testing.T) {
		if _, err := newGW(base, true).Validate(context.Background(), "", "gpt-5.4"); err == nil {
			t.Fatal("expected error for empty provider")
		}
	})
	t.Run("unknown provider", func(t *testing.T) {
		if _, err := newGW(base, false).Validate(context.Background(), "nope", ""); err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
	t.Run("disabled provider", func(t *testing.T) {
		p := base
		p.Enabled = false
		if _, err := newGW(p, true).Validate(context.Background(), "openai", ""); err == nil {
			t.Fatal("expected error for disabled provider")
		}
	})
	t.Run("no key", func(t *testing.T) {
		p := base
		p.APIKeyEnc = nil
		if _, err := newGW(p, true).Validate(context.Background(), "openai", ""); err == nil {
			t.Fatal("expected error for provider without key")
		}
	})
	t.Run("empty model ok and unpriced", func(t *testing.T) {
		price, err := newGW(base, true).Validate(context.Background(), "openai", "")
		if err != nil {
			t.Fatalf("empty model should be allowed: %v", err)
		}
		if price.Priced() {
			t.Error("no model pinned => unpriced snapshot")
		}
	})
	t.Run("enabled model freezes price", func(t *testing.T) {
		price, err := newGW(base, true).Validate(context.Background(), "OpenAI", "gpt-5.4")
		if err != nil {
			t.Fatalf("enabled model should pass: %v", err)
		}
		cost, ok := price.Cost(1_000_000, 1_000_000)
		if !ok {
			t.Fatal("priced model must return a usable price snapshot")
		}
		if cost != priceIn+priceOut {
			t.Errorf("frozen cost = %v, want %v", cost, priceIn+priceOut)
		}
	})
	t.Run("enabled model without price is unpriced", func(t *testing.T) {
		price, err := newGW(base, true).Validate(context.Background(), "openai", "gpt-noprice")
		if err != nil {
			t.Fatalf("priceless model should still validate: %v", err)
		}
		if price.Priced() {
			t.Error("model without configured price must be unpriced, not priced-zero")
		}
	})
	t.Run("disabled model rejected", func(t *testing.T) {
		if _, err := newGW(base, true).Validate(context.Background(), "openai", "gpt-old"); err == nil {
			t.Fatal("expected error for disabled model")
		}
	})
	t.Run("unknown model rejected", func(t *testing.T) {
		if _, err := newGW(base, true).Validate(context.Background(), "openai", "gpt-nonexistent"); err == nil {
			t.Fatal("expected error for unknown model")
		}
	})
}

func TestDBGatewayRebuildsOnVersionChange(t *testing.T) {
	repo := &fakeRepo{found: true, provider: domain.LLMProvider{
		Name: "OpenAI", AdapterType: domain.AdapterOpenAICompatible,
		APIKeyEnc: []byte("secret"), Enabled: true,
	}}
	g := NewDBGateway(repo, fakeCipher{})

	a, _ := g.resolve(context.Background(), "openai")

	// Simulate an admin edit: bump UpdatedAt so the cached version is stale.
	repo.provider.UpdatedAt = repo.provider.UpdatedAt.Add(1)
	b, _ := g.resolve(context.Background(), "openai")
	if a == b {
		t.Fatal("expected adapter to be rebuilt after provider version changed")
	}
}
