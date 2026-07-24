package db

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"apant_be/internal/domain"
)

// MemoryLLMRepository is an in-memory LLMProviderRepository for tests and
// STORAGE=memory runs. Models are kept flattened and re-attached on read.
type MemoryLLMRepository struct {
	mu        sync.RWMutex
	providers map[string]domain.LLMProvider
	models    map[string]domain.LLMModel
}

func NewMemoryLLMRepository() *MemoryLLMRepository {
	return &MemoryLLMRepository{
		providers: make(map[string]domain.LLMProvider),
		models:    make(map[string]domain.LLMModel),
	}
}

func (r *MemoryLLMRepository) modelsFor(providerID string) []domain.LLMModel {
	out := make([]domain.LLMModel, 0)
	for _, m := range r.models {
		if m.ProviderID == providerID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (r *MemoryLLMRepository) ListProviders(_ context.Context) ([]domain.LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.LLMProvider, 0, len(r.providers))
	for _, p := range r.providers {
		p.Models = r.modelsFor(p.ID)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (r *MemoryLLMRepository) GetProvider(_ context.Context, id string) (domain.LLMProvider, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[strings.TrimSpace(id)]
	if !ok {
		return domain.LLMProvider{}, false, nil
	}
	p.Models = r.modelsFor(p.ID)
	return p, true, nil
}

func (r *MemoryLLMRepository) GetProviderByName(_ context.Context, name string) (domain.LLMProvider, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name = strings.TrimSpace(name)
	for _, p := range r.providers {
		if strings.EqualFold(p.Name, name) {
			p.Models = r.modelsFor(p.ID)
			return p, true, nil
		}
	}
	return domain.LLMProvider{}, false, nil
}

func (r *MemoryLLMRepository) CreateProvider(_ context.Context, p domain.LLMProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("provider id is required")
	}
	for _, existing := range r.providers {
		if strings.EqualFold(existing.Name, strings.TrimSpace(p.Name)) {
			return fmt.Errorf("a provider with that name already exists")
		}
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	p.Models = nil
	r.providers[p.ID] = p
	return nil
}

func (r *MemoryLLMRepository) UpdateProvider(_ context.Context, p domain.LLMProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.providers[strings.TrimSpace(p.ID)]
	if !ok {
		return domain.ErrLLMProviderNotFound
	}
	existing.Name = strings.TrimSpace(p.Name)
	existing.AdapterType = p.AdapterType
	existing.BaseURL = strings.TrimSpace(p.BaseURL)
	existing.Enabled = p.Enabled
	existing.UpdatedAt = time.Now()
	if p.APIKeyEnc != nil {
		existing.APIKeyEnc = p.APIKeyEnc
	}
	r.providers[existing.ID] = existing
	return nil
}

func (r *MemoryLLMRepository) DeleteProvider(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id = strings.TrimSpace(id)
	delete(r.providers, id)
	for mid, m := range r.models {
		if m.ProviderID == id {
			delete(r.models, mid)
		}
	}
	return nil
}

func (r *MemoryLLMRepository) AddModel(_ context.Context, m domain.LLMModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.ProviderID) == "" {
		return fmt.Errorf("model id and provider id are required")
	}
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	r.models[m.ID] = m
	return nil
}

func (r *MemoryLLMRepository) UpdateModel(_ context.Context, m domain.LLMModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.models[strings.TrimSpace(m.ID)]
	if !ok {
		return fmt.Errorf("model not found")
	}
	existing.ModelID = strings.TrimSpace(m.ModelID)
	existing.Label = strings.TrimSpace(m.Label)
	existing.Enabled = m.Enabled
	existing.PriceInPer1M = m.PriceInPer1M
	existing.PriceOutPer1M = m.PriceOutPer1M
	existing.Currency = m.Currency
	existing.UpdatedAt = time.Now()
	r.models[existing.ID] = existing
	return nil
}

func (r *MemoryLLMRepository) DeleteModel(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.models, strings.TrimSpace(id))
	return nil
}

func (r *MemoryLLMRepository) CountProviders(_ context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int64(len(r.providers)), nil
}
