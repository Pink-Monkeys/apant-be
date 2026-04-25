package ai

import (
	"context"

	"apant_be/internal/domain"
)

// Provider is a concrete AI backend adapter (OpenAI, Claude, etc).
type Provider interface {
	Name() string
	Generate(ctx context.Context, in domain.AIGenerateInput) (domain.AIGenerateOutput, error)
}
