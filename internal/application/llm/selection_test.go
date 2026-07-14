package llm

import (
	"context"
	"testing"

	"apant_be/internal/domain"
)

// memPrefs is an in-test UserLLMPreferenceRepository double, kept in the
// application package so the test does not import the infrastructure layer.
type memPrefs struct {
	m map[string]domain.UserLLMPreference
}

func newMemPrefs() *memPrefs { return &memPrefs{m: make(map[string]domain.UserLLMPreference)} }

func (r *memPrefs) Get(_ context.Context, userID string) (domain.UserLLMPreference, bool, error) {
	p, ok := r.m[userID]
	return p, ok, nil
}
func (r *memPrefs) Upsert(_ context.Context, pref domain.UserLLMPreference) error {
	r.m[pref.UserID] = pref
	return nil
}

// fakeProviderRepo returns a fixed catalog; only ListProviders is exercised by
// the selection tests. The rest satisfy the interface.
type fakeProviderRepo struct {
	providers []domain.LLMProvider
}

func (r *fakeProviderRepo) ListProviders(context.Context) ([]domain.LLMProvider, error) {
	return r.providers, nil
}
func (r *fakeProviderRepo) GetProvider(context.Context, string) (domain.LLMProvider, bool, error) {
	return domain.LLMProvider{}, false, nil
}
func (r *fakeProviderRepo) GetProviderByName(context.Context, string) (domain.LLMProvider, bool, error) {
	return domain.LLMProvider{}, false, nil
}
func (r *fakeProviderRepo) CreateProvider(context.Context, domain.LLMProvider) error { return nil }
func (r *fakeProviderRepo) UpdateProvider(context.Context, domain.LLMProvider) error { return nil }
func (r *fakeProviderRepo) DeleteProvider(context.Context, string) error             { return nil }
func (r *fakeProviderRepo) AddModel(context.Context, domain.LLMModel) error          { return nil }
func (r *fakeProviderRepo) UpdateModel(context.Context, domain.LLMModel) error       { return nil }
func (r *fakeProviderRepo) DeleteModel(context.Context, string) error                { return nil }
func (r *fakeProviderRepo) CountProviders(context.Context) (int64, error)            { return 0, nil }

// catalog builds a provider with enabled models. APIKeyEnc must be non-empty for
// Options() to include the provider.
func catalog(name string, models ...string) domain.LLMProvider {
	p := domain.LLMProvider{Name: name, AdapterType: domain.AdapterOpenAIChat, Enabled: true, APIKeyEnc: []byte("x")}
	for _, m := range models {
		p.Models = append(p.Models, domain.LLMModel{ModelID: m, Enabled: true})
	}
	return p
}

func newSelectionService(providers []domain.LLMProvider) *Service {
	return &Service{
		repo:  &fakeProviderRepo{providers: providers},
		prefs: newMemPrefs(),
	}
}

// TestSetAndGetSelection_Roundtrip: a valid selection is stored and returned.
func TestSetAndGetSelection_Roundtrip(t *testing.T) {
	s := newSelectionService([]domain.LLMProvider{catalog("openai", "gpt-5.4", "gpt-4o")})

	sel, err := s.SetSelection(context.Background(), "u1", "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("SetSelection: %v", err)
	}
	if sel.Provider != "openai" || sel.Model != "gpt-4o" {
		t.Errorf("SetSelection returned %+v, want openai/gpt-4o", sel)
	}

	got, err := s.GetSelection(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetSelection: %v", err)
	}
	if got.Provider != "openai" || got.Model != "gpt-4o" {
		t.Errorf("GetSelection returned %+v, want openai/gpt-4o", got)
	}
}

// TestSetSelection_RejectsUnavailable: a provider/model not in the catalog is
// rejected rather than stored.
func TestSetSelection_RejectsUnavailable(t *testing.T) {
	s := newSelectionService([]domain.LLMProvider{catalog("openai", "gpt-5.4")})

	if _, err := s.SetSelection(context.Background(), "u1", "openai", "does-not-exist"); err == nil {
		t.Error("expected error for unavailable model, got nil")
	}
	if _, err := s.SetSelection(context.Background(), "u1", "unknown-provider", "gpt-5.4"); err == nil {
		t.Error("expected error for unknown provider, got nil")
	}
}

// TestGetSelection_FallbackWhenModelRemoved: a stored model that is later
// removed from the catalog falls back to the first available option.
func TestGetSelection_FallbackWhenModelRemoved(t *testing.T) {
	s := newSelectionService([]domain.LLMProvider{catalog("openai", "gpt-5.4", "gpt-4o")})
	if _, err := s.SetSelection(context.Background(), "u1", "openai", "gpt-4o"); err != nil {
		t.Fatalf("SetSelection: %v", err)
	}

	// Admin removes gpt-4o; only gpt-5.4 remains.
	s.repo = &fakeProviderRepo{providers: []domain.LLMProvider{catalog("openai", "gpt-5.4")}}

	got, err := s.GetSelection(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetSelection: %v", err)
	}
	if got.Provider != "openai" || got.Model != "gpt-5.4" {
		t.Errorf("stale selection not replaced: got %+v, want openai/gpt-5.4 (first available)", got)
	}
}

// TestGetSelection_DefaultWhenNoneSet: a user who never chose gets the first
// available option, not an empty selection.
func TestGetSelection_DefaultWhenNoneSet(t *testing.T) {
	s := newSelectionService([]domain.LLMProvider{catalog("anthropic", "claude-x"), catalog("openai", "gpt-5.4")})

	got, err := s.GetSelection(context.Background(), "fresh-user")
	if err != nil {
		t.Fatalf("GetSelection: %v", err)
	}
	if got.Provider != "anthropic" || got.Model != "claude-x" {
		t.Errorf("default = %+v, want anthropic/claude-x (first catalog option)", got)
	}
}

// TestGetSelection_PerUserIsolation: two users keep independent selections —
// the exact bug this feature fixes.
func TestGetSelection_PerUserIsolation(t *testing.T) {
	s := newSelectionService([]domain.LLMProvider{catalog("openai", "gpt-5.4", "gpt-4o")})

	if _, err := s.SetSelection(context.Background(), "userA", "openai", "gpt-5.4"); err != nil {
		t.Fatalf("SetSelection A: %v", err)
	}
	if _, err := s.SetSelection(context.Background(), "userB", "openai", "gpt-4o"); err != nil {
		t.Fatalf("SetSelection B: %v", err)
	}

	a, _ := s.GetSelection(context.Background(), "userA")
	b, _ := s.GetSelection(context.Background(), "userB")
	if a.Model != "gpt-5.4" {
		t.Errorf("userA selection leaked: got %q, want gpt-5.4", a.Model)
	}
	if b.Model != "gpt-4o" {
		t.Errorf("userB selection leaked: got %q, want gpt-4o", b.Model)
	}
}
