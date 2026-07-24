package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"apant_be/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type llmProviderModel struct {
	ID          string    `gorm:"column:id;type:text;primaryKey"`
	Name        string    `gorm:"column:name;type:text;uniqueIndex;not null"`
	AdapterType string    `gorm:"column:adapter_type;type:text;not null"`
	APIKeyEnc   []byte    `gorm:"column:api_key_enc;type:bytea"`
	BaseURL     string    `gorm:"column:base_url;type:text"`
	Enabled     bool      `gorm:"column:enabled;not null;default:true"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

func (llmProviderModel) TableName() string { return "llm_providers" }

type llmModelModel struct {
	ID         string `gorm:"column:id;type:text;primaryKey"`
	ProviderID string `gorm:"column:provider_id;type:text;index;not null"`
	ModelID    string `gorm:"column:model_id;type:text;not null"`
	Label      string `gorm:"column:label;type:text"`
	Enabled    bool   `gorm:"column:enabled;not null;default:true"`
	// Per-1M-token prices. Nullable so "unpriced" (NULL) stays distinct from
	// "free" (0). Pointers map NULL<->nil without a float64 zero-value collision.
	PriceInPer1M  *float64  `gorm:"column:price_in_per_1m;type:numeric"`
	PriceOutPer1M *float64  `gorm:"column:price_out_per_1m;type:numeric"`
	Currency      *string   `gorm:"column:currency;type:text"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

func (llmModelModel) TableName() string { return "llm_models" }

// Compile-time checks that both backends satisfy the repository port.
var (
	_ domain.LLMProviderRepository = (*LLMRepositoryPostgres)(nil)
	_ domain.LLMProviderRepository = (*MemoryLLMRepository)(nil)
)

type LLMRepositoryPostgres struct {
	db *Postgres
}

func NewLLMRepositoryPostgres(db *Postgres) (*LLMRepositoryPostgres, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("postgres db is not initialized")
	}
	return &LLMRepositoryPostgres{db: db}, nil
}

func (r *LLMRepositoryPostgres) ListProviders(ctx context.Context) ([]domain.LLMProvider, error) {
	var providers []llmProviderModel
	if err := r.db.DB.WithContext(ctx).Order("created_at ASC").Find(&providers).Error; err != nil {
		return nil, err
	}

	var models []llmModelModel
	if err := r.db.DB.WithContext(ctx).Order("created_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	byProvider := make(map[string][]domain.LLMModel, len(providers))
	for _, m := range models {
		byProvider[m.ProviderID] = append(byProvider[m.ProviderID], toDomainLLMModel(m))
	}

	out := make([]domain.LLMProvider, 0, len(providers))
	for _, p := range providers {
		dp := toDomainLLMProvider(p)
		dp.Models = byProvider[p.ID]
		out = append(out, dp)
	}
	return out, nil
}

func (r *LLMRepositoryPostgres) GetProvider(ctx context.Context, id string) (domain.LLMProvider, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.LLMProvider{}, false, nil
	}
	return r.getProviderWhere(ctx, "id = ?", id)
}

func (r *LLMRepositoryPostgres) GetProviderByName(ctx context.Context, name string) (domain.LLMProvider, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.LLMProvider{}, false, nil
	}
	// Case-insensitive: the scan request's provider field is lowercased by the
	// caller ("openai") but the stored display name may be "OpenAI".
	return r.getProviderWhere(ctx, "LOWER(name) = LOWER(?)", name)
}

func (r *LLMRepositoryPostgres) getProviderWhere(ctx context.Context, query string, arg any) (domain.LLMProvider, bool, error) {
	var p llmProviderModel
	if err := r.db.DB.WithContext(ctx).Where(query, arg).First(&p).Error; err != nil {
		if isNotFound(err) {
			return domain.LLMProvider{}, false, nil
		}
		return domain.LLMProvider{}, false, err
	}

	var models []llmModelModel
	if err := r.db.DB.WithContext(ctx).Where("provider_id = ?", p.ID).Order("created_at ASC").Find(&models).Error; err != nil {
		return domain.LLMProvider{}, false, err
	}

	dp := toDomainLLMProvider(p)
	for _, m := range models {
		dp.Models = append(dp.Models, toDomainLLMModel(m))
	}
	return dp, true, nil
}

func (r *LLMRepositoryPostgres) CreateProvider(ctx context.Context, p domain.LLMProvider) error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("provider id is required")
	}
	model := fromDomainLLMProvider(p)
	if err := r.db.DB.WithContext(ctx).Create(&model).Error; err != nil {
		if isUniqueViolation(err, "") {
			return fmt.Errorf("a provider with that name already exists")
		}
		return err
	}
	return nil
}

func (r *LLMRepositoryPostgres) UpdateProvider(ctx context.Context, p domain.LLMProvider) error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("provider id is required")
	}

	updates := map[string]any{
		"name":         strings.TrimSpace(p.Name),
		"adapter_type": p.AdapterType,
		"base_url":     strings.TrimSpace(p.BaseURL),
		"enabled":      p.Enabled,
		"updated_at":   time.Now(),
	}
	// Only overwrite the stored key when a new one was supplied, so editing other
	// fields does not wipe an existing key.
	if p.APIKeyEnc != nil {
		updates["api_key_enc"] = p.APIKeyEnc
	}

	res := r.db.DB.WithContext(ctx).Model(&llmProviderModel{}).Where("id = ?", p.ID).Updates(updates)
	if res.Error != nil {
		if isUniqueViolation(res.Error, "") {
			return fmt.Errorf("a provider with that name already exists")
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrLLMProviderNotFound
	}
	return nil
}

func (r *LLMRepositoryPostgres) DeleteProvider(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("provider id is required")
	}
	// Cascade the delete to owned models in one transaction.
	return r.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", id).Delete(&llmModelModel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&llmProviderModel{}).Error
	})
}

func (r *LLMRepositoryPostgres) AddModel(ctx context.Context, m domain.LLMModel) error {
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.ProviderID) == "" {
		return fmt.Errorf("model id and provider id are required")
	}
	model := fromDomainLLMModel(m)
	return r.db.DB.WithContext(ctx).Create(&model).Error
}

func (r *LLMRepositoryPostgres) UpdateModel(ctx context.Context, m domain.LLMModel) error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("model id is required")
	}
	// The service passes the fully-resolved target (price already patched or
	// cleared), so we set the three price columns from it directly: a nil pointer
	// writes NULL (unpriced), a non-nil value writes the number (0 => free).
	updates := map[string]any{
		"model_id":         strings.TrimSpace(m.ModelID),
		"label":            strings.TrimSpace(m.Label),
		"enabled":          m.Enabled,
		"price_in_per_1m":  m.PriceInPer1M,
		"price_out_per_1m": m.PriceOutPer1M,
		"currency":         m.Currency,
		"updated_at":       time.Now(),
	}
	return r.db.DB.WithContext(ctx).Model(&llmModelModel{}).Where("id = ?", m.ID).Updates(updates).Error
}

func (r *LLMRepositoryPostgres) DeleteModel(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("model id is required")
	}
	return r.db.DB.WithContext(ctx).Where("id = ?", id).Delete(&llmModelModel{}).Error
}

func (r *LLMRepositoryPostgres) CountProviders(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.DB.WithContext(ctx).Model(&llmProviderModel{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// upsertProviderTx is used by the seed migration to insert a provider + its
// models idempotently.
func upsertProviderTx(tx *gorm.DB, p domain.LLMProvider) error {
	model := fromDomainLLMProvider(p)
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model).Error; err != nil {
		return err
	}
	for _, m := range p.Models {
		mm := fromDomainLLMModel(m)
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&mm).Error; err != nil {
			return err
		}
	}
	return nil
}

func toDomainLLMProvider(m llmProviderModel) domain.LLMProvider {
	return domain.LLMProvider{
		ID:          m.ID,
		Name:        m.Name,
		AdapterType: m.AdapterType,
		APIKeyEnc:   m.APIKeyEnc,
		BaseURL:     m.BaseURL,
		Enabled:     m.Enabled,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func fromDomainLLMProvider(p domain.LLMProvider) llmProviderModel {
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	return llmProviderModel{
		ID:          p.ID,
		Name:        strings.TrimSpace(p.Name),
		AdapterType: p.AdapterType,
		APIKeyEnc:   p.APIKeyEnc,
		BaseURL:     strings.TrimSpace(p.BaseURL),
		Enabled:     p.Enabled,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func toDomainLLMModel(m llmModelModel) domain.LLMModel {
	return domain.LLMModel{
		ID:            m.ID,
		ProviderID:    m.ProviderID,
		ModelID:       m.ModelID,
		Label:         m.Label,
		Enabled:       m.Enabled,
		PriceInPer1M:  m.PriceInPer1M,
		PriceOutPer1M: m.PriceOutPer1M,
		Currency:      m.Currency,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func fromDomainLLMModel(m domain.LLMModel) llmModelModel {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return llmModelModel{
		ID:            m.ID,
		ProviderID:    m.ProviderID,
		ModelID:       strings.TrimSpace(m.ModelID),
		Label:         strings.TrimSpace(m.Label),
		Enabled:       m.Enabled,
		PriceInPer1M:  m.PriceInPer1M,
		PriceOutPer1M: m.PriceOutPer1M,
		Currency:      m.Currency,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func migrateLLM(tx *gorm.DB) error {
	return tx.AutoMigrate(&llmProviderModel{}, &llmModelModel{})
}
