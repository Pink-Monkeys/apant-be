package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"apant_be/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Compile-time checks that both backends satisfy the repository port.
var (
	_ domain.UserLLMPreferenceRepository = (*PostgresUserLLMPreferenceRepository)(nil)
	_ domain.UserLLMPreferenceRepository = (*MemoryUserLLMPreferenceRepository)(nil)
)

type userLLMPreferenceModel struct {
	UserID    string    `gorm:"column:user_id;type:text;primaryKey"`
	Provider  string    `gorm:"column:provider;type:text;not null"`
	Model     string    `gorm:"column:model;type:text;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (userLLMPreferenceModel) TableName() string { return "user_llm_preferences" }

// --- Postgres ---

type PostgresUserLLMPreferenceRepository struct {
	db *Postgres
}

func NewPostgresUserLLMPreferenceRepository(db *Postgres) (*PostgresUserLLMPreferenceRepository, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("postgres db is not initialized")
	}
	return &PostgresUserLLMPreferenceRepository{db: db}, nil
}

func (r *PostgresUserLLMPreferenceRepository) Get(ctx context.Context, userID string) (domain.UserLLMPreference, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.UserLLMPreference{}, false, nil
	}

	var model userLLMPreferenceModel
	if err := r.db.DB.WithContext(ctx).Where("user_id = ?", userID).First(&model).Error; err != nil {
		if isNotFound(err) {
			return domain.UserLLMPreference{}, false, nil
		}
		return domain.UserLLMPreference{}, false, err
	}

	return domain.UserLLMPreference{
		UserID:    model.UserID,
		Provider:  model.Provider,
		Model:     model.Model,
		UpdatedAt: model.UpdatedAt,
	}, true, nil
}

func (r *PostgresUserLLMPreferenceRepository) Upsert(ctx context.Context, pref domain.UserLLMPreference) error {
	userID := strings.TrimSpace(pref.UserID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}

	model := userLLMPreferenceModel{
		UserID:    userID,
		Provider:  strings.TrimSpace(pref.Provider),
		Model:     strings.TrimSpace(pref.Model),
		UpdatedAt: time.Now(),
	}

	// One row per user: replace provider/model/updated_at on conflict.
	return r.db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"provider", "model", "updated_at"}),
	}).Create(&model).Error
}

// --- Memory ---

type MemoryUserLLMPreferenceRepository struct {
	mu    sync.RWMutex
	prefs map[string]domain.UserLLMPreference
}

func NewMemoryUserLLMPreferenceRepository() *MemoryUserLLMPreferenceRepository {
	return &MemoryUserLLMPreferenceRepository{prefs: make(map[string]domain.UserLLMPreference)}
}

func (r *MemoryUserLLMPreferenceRepository) Get(_ context.Context, userID string) (domain.UserLLMPreference, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.UserLLMPreference{}, false, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	pref, ok := r.prefs[userID]
	return pref, ok, nil
}

func (r *MemoryUserLLMPreferenceRepository) Upsert(_ context.Context, pref domain.UserLLMPreference) error {
	userID := strings.TrimSpace(pref.UserID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefs[userID] = domain.UserLLMPreference{
		UserID:    userID,
		Provider:  strings.TrimSpace(pref.Provider),
		Model:     strings.TrimSpace(pref.Model),
		UpdatedAt: time.Now(),
	}
	return nil
}

func migrateUserLLMPreferences(tx *gorm.DB) error {
	return tx.AutoMigrate(&userLLMPreferenceModel{})
}
