package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"apant_be/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type scanModel struct {
	ID          string    `gorm:"column:id;type:text;primaryKey"`
	SessionID   string    `gorm:"column:session_id;type:text;index"`
	UserID      string    `gorm:"column:user_id;type:text;index"`
	Target      string    `gorm:"column:target;type:text"`
	Provider    string    `gorm:"column:provider;type:text"`
	Model       string    `gorm:"column:model;type:text"`
	Message     string    `gorm:"column:message;type:text"`
	Description string    `gorm:"column:description;type:text"`
	ScanType    string    `gorm:"column:scan_type;type:text"`
	Status      string    `gorm:"column:status;type:text"`
	Steps       []byte    `gorm:"column:steps;type:jsonb"`
	FinalAnswer string    `gorm:"column:final_answer;type:text"`
	Error       string    `gorm:"column:error;type:text"`
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
		Description: scan.Description,
		ScanType:    scan.ScanType,
		Status:      scan.Status,
		Steps:       steps,
		FinalAnswer: scan.FinalAnswer,
		Error:       scan.Error,
		Duration:    scan.Duration,
		TargetInfo:  targetInfo,
		CreatedAt:   scan.CreatedAt,
		UpdatedAt:   scan.UpdatedAt,
	}

	// Upsert: a scan is first saved as "running" then re-saved as
	// "completed"/"failed" by the background static scanner, so Save must be
	// idempotent on the primary key rather than failing on conflict.
	return r.db.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&model).Error
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
		Description: model.Description,
		ScanType:    model.ScanType,
		Status:      model.Status,
		Steps:       steps,
		FinalAnswer: model.FinalAnswer,
		Error:       model.Error,
		Duration:    model.Duration,
		TargetInfo:  targetInfo,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func (r *ScanRepositoryPostgres) FindRunningByUserID(ctx context.Context, userID string) (domain.Scan, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.Scan{}, false, nil
	}

	var model scanModel
	err := r.db.DB.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, domain.ScanStatusRunning).
		Order("created_at DESC").
		First(&model).Error
	if err != nil {
		if isNotFound(err) {
			return domain.Scan{}, false, nil
		}
		return domain.Scan{}, false, err
	}

	scan, err := toDomainScan(model)
	if err != nil {
		return domain.Scan{}, false, err
	}
	return scan, true, nil
}

func (r *ScanRepositoryPostgres) FailStaleRunning(ctx context.Context, olderThan time.Time, reason string) (int64, error) {
	res := r.db.DB.WithContext(ctx).
		Model(&scanModel{}).
		Where("status = ? AND created_at < ?", domain.ScanStatusRunning, olderThan).
		Updates(map[string]any{
			"status":     domain.ScanStatusFailed,
			"error":      reason,
			"updated_at": time.Now(),
		})
	return res.RowsAffected, res.Error
}

func migrateScans(tx *gorm.DB) error {
	return tx.AutoMigrate(&scanModel{})
}
