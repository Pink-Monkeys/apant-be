package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"apant_be/internal/domain"

	"gorm.io/gorm"
)

type scanModel struct {
	ID          string    `gorm:"column:id;type:text;primaryKey"`
	SessionID   string    `gorm:"column:session_id;type:text;index"`
	UserID      string    `gorm:"column:user_id;type:text;index"`
	Target      string    `gorm:"column:target;type:text"`
	Provider    string    `gorm:"column:provider;type:text"`
	Model       string    `gorm:"column:model;type:text"`
	Message     string    `gorm:"column:message;type:text"`
	Status      string    `gorm:"column:status;type:text"`
	Steps       []byte    `gorm:"column:steps;type:jsonb"`
	FinalAnswer string    `gorm:"column:final_answer;type:text"`
	Duration    string    `gorm:"column:duration;type:text"`
	TargetInfo  []byte    `gorm:"column:target_info;type:jsonb"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (scanModel) TableName() string {
	return "scans"
}

type ScanRepositoryPostgres struct {
	db *Postgres
}

func NewScanRepositoryPostgres(db *Postgres) (*ScanRepositoryPostgres, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("postgres db is not initialized")
	}

	return &ScanRepositoryPostgres{db: db}, nil
}

func (r *ScanRepositoryPostgres) Save(ctx context.Context, scan domain.Scan) error {
	if strings.TrimSpace(scan.ID) == "" {
		return fmt.Errorf("scan id is required")
	}

	steps, err := json.Marshal(scan.Steps)
	if err != nil {
		return fmt.Errorf("marshal scan steps: %w", err)
	}

	var targetInfo []byte
	if scan.TargetInfo != nil {
		targetInfo, err = json.Marshal(scan.TargetInfo)
		if err != nil {
			return fmt.Errorf("marshal scan target info: %w", err)
		}
	}

	model := scanModel{
		ID:          scan.ID,
		SessionID:   strings.TrimSpace(scan.SessionID),
		UserID:      strings.TrimSpace(scan.UserID),
		Target:      scan.Target,
		Provider:    scan.Provider,
		Model:       scan.Model,
		Message:     scan.Message,
		Status:      scan.Status,
		Steps:       steps,
		FinalAnswer: scan.FinalAnswer,
		Duration:    scan.Duration,
		TargetInfo:  targetInfo,
		CreatedAt:   scan.CreatedAt,
		UpdatedAt:   scan.UpdatedAt,
	}

	return r.db.DB.WithContext(ctx).Create(&model).Error
}

func (r *ScanRepositoryPostgres) FindByID(ctx context.Context, id string) (domain.Scan, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Scan{}, fmt.Errorf("scan id is required")
	}

	var model scanModel
	if err := r.db.DB.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if isNotFound(err) {
			return domain.Scan{}, fmt.Errorf("scan not found")
		}
		return domain.Scan{}, err
	}

	return toDomainScan(model)
}

func (r *ScanRepositoryPostgres) FindByUserID(ctx context.Context, userID string) ([]domain.Scan, error) {
	userID = strings.TrimSpace(userID)
	var models []scanModel

	query := r.db.DB.WithContext(ctx).Order("created_at DESC")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	out := make([]domain.Scan, 0, len(models))
	for _, model := range models {
		scan, err := toDomainScan(model)
		if err != nil {
			continue
		}
		out = append(out, scan)
	}

	return out, nil
}

func toDomainScan(model scanModel) (domain.Scan, error) {
	steps := make([]domain.ScanStep, 0)
	if len(model.Steps) > 0 {
		if err := json.Unmarshal(model.Steps, &steps); err != nil {
			return domain.Scan{}, fmt.Errorf("unmarshal scan steps: %w", err)
		}
	}

	var targetInfo *domain.TargetInfo
	if len(model.TargetInfo) > 0 {
		var ti domain.TargetInfo
		if err := json.Unmarshal(model.TargetInfo, &ti); err == nil {
			targetInfo = &ti
		}
	}

	return domain.Scan{
		ID:          model.ID,
		SessionID:   model.SessionID,
		UserID:      model.UserID,
		Target:      model.Target,
		Provider:    model.Provider,
		Model:       model.Model,
		Message:     model.Message,
		Status:      model.Status,
		Steps:       steps,
		FinalAnswer: model.FinalAnswer,
		Duration:    model.Duration,
		TargetInfo:  targetInfo,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func migrateScans(tx *gorm.DB) error {
	return tx.AutoMigrate(&scanModel{})
}
