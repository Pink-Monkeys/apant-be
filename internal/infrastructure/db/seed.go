package db

import (
	"context"
	"strings"
	"time"

	"apant_be/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// seedLLMFromEnv populates llm_providers/llm_models from the legacy env-based
// config on first run, so the system is not empty after the feature ships.
// It is idempotent (skips when any provider already exists) and a no-op when no
// encryption key is configured or no env keys are present.
func seedLLMFromEnv(tx *gorm.DB, seed SeedConfig) error {
	if seed.EncryptKey == nil {
		return nil
	}

	var count int64
	if err := tx.Model(&llmProviderModel{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	if key := strings.TrimSpace(seed.OpenAIAPIKey); key != "" {
		enc, err := seed.EncryptKey(key)
		if err != nil {
			return err
		}
		p := domain.LLMProvider{
			ID:          uuid.NewString(),
			Name:        "OpenAI",
			AdapterType: domain.AdapterOpenAICompatible,
			APIKeyEnc:   enc,
			Enabled:     true,
		}
		if m := strings.TrimSpace(seed.OpenAIModel); m != "" {
			p.Models = []domain.LLMModel{{
				ID:         uuid.NewString(),
				ProviderID: p.ID,
				ModelID:    m,
				Enabled:    true,
			}}
		}
		if err := upsertProviderTx(tx, p); err != nil {
			return err
		}
	}

	if key := strings.TrimSpace(seed.AnthropicKey); key != "" {
		enc, err := seed.EncryptKey(key)
		if err != nil {
			return err
		}
		p := domain.LLMProvider{
			ID:          uuid.NewString(),
			Name:        "Anthropic",
			AdapterType: domain.AdapterAnthropic,
			APIKeyEnc:   enc,
			Enabled:     true,
		}
		if m := strings.TrimSpace(seed.AnthropicModel); m != "" {
			p.Models = []domain.LLMModel{{
				ID:         uuid.NewString(),
				ProviderID: p.ID,
				ModelID:    m,
				Enabled:    true,
			}}
		}
		if err := upsertProviderTx(tx, p); err != nil {
			return err
		}
	}

	return nil
}

// seedAdminUser creates a single bootstrap admin if no admin exists yet. The
// password hash is supplied pre-computed by the caller so this package needs no
// auth dependency. No-op when the seed omits admin credentials.
func seedAdminUser(tx *gorm.DB, seed SeedConfig) error {
	username := strings.ToLower(strings.TrimSpace(seed.AdminUsername))
	if username == "" || strings.TrimSpace(seed.AdminPasswordHash) == "" {
		return nil
	}

	var count int64
	if err := tx.Model(&userModel{}).Where("role = ?", domain.RoleAdmin).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	admin := userModel{
		ID:           uuid.NewString(),
		Username:     username,
		Email:        strings.ToLower(strings.TrimSpace(seed.AdminEmail)),
		PasswordHash: seed.AdminPasswordHash,
		Role:         domain.RoleAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	// DoNothing guards the rare race where a same-username user exists.
	return tx.WithContext(context.Background()).Create(&admin).Error
}
