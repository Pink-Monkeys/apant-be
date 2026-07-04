package ai

import (
	"context"
	"fmt"

	"apant_be/internal/domain"
)

type Gateway struct {
	providers map[string]Provider
}

func NewGateway(items ...Provider) *Gateway {
	m := make(map[string]Provider)
	for _, p := range items {
		m[p.Name()] = p
	}
	return &Gateway{providers: m}
}

func (g *Gateway) Generate(ctx context.Context, provider string, in domain.AIGenerateInput) (domain.AIGenerateOutput, error) {
	p, ok := g.providers[provider]
	if !ok {
		return domain.AIGenerateOutput{}, fmt.Errorf("unsupported provider: %s", provider)
	}
	return p.Generate(ctx, in)
}

// Validate checks only that the provider is one of the statically-wired
// backends. This gateway has no model catalog (it is the no-DB fallback), so any
// model is accepted.
func (g *Gateway) Validate(_ context.Context, provider, _ string) error {
	if _, ok := g.providers[provider]; !ok {
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	return nil
}
