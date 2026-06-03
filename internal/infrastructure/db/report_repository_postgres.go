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

type reportModel struct {
	ID        string    `gorm:"column:id;type:text;primaryKey"`
	SessionID string    `gorm:"column:session_id;type:text;index"`
	UserID    string    `gorm:"column:user_id;type:text;index"`
	Data      []byte    `gorm:"column:data;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (reportModel) TableName() string {
	return "reports"
}

type PostgresReportRepository struct {
	db *Postgres
}

func NewPostgresReportRepository(db *Postgres) (*PostgresReportRepository, error) {
	if db == nil || db.DB == nil {
		return nil, fmt.Errorf("postgres db is not initialized")
	}

	return &PostgresReportRepository{db: db}, nil
}

func (r *PostgresReportRepository) Save(ctx context.Context, report domain.Report) error {
	if strings.TrimSpace(report.ID) == "" {
		return fmt.Errorf("report id is required")
	}

	data, err := json.Marshal(report.Data)
	if err != nil {
		return fmt.Errorf("marshal report data: %w", err)
	}

	model := reportModel{
		ID:        report.ID,
		SessionID: strings.TrimSpace(report.SessionID),
		UserID:    strings.TrimSpace(report.UserID),
		Data:      data,
		CreatedAt: report.CreatedAt,
	}

	return r.db.DB.WithContext(ctx).Create(&model).Error
}

func (r *PostgresReportRepository) FindByID(ctx context.Context, id string) (domain.Report, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Report{}, fmt.Errorf("report id is required")
	}

	var model reportModel
	if err := r.db.DB.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if isNotFound(err) {
			return domain.Report{}, fmt.Errorf("report not found")
		}
		return domain.Report{}, err
	}

	return toDomainReport(model)
}

func (r *PostgresReportRepository) FindByUserID(ctx context.Context, userID string) ([]domain.Report, error) {
	userID = strings.TrimSpace(userID)
	var models []reportModel

	query := r.db.DB.WithContext(ctx).Order("created_at DESC")
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	out := make([]domain.Report, 0, len(models))
	for _, model := range models {
		report, err := toDomainReport(model)
		if err != nil {
			continue
		}
		out = append(out, report)
	}

	return out, nil
}

func (r *PostgresReportRepository) DeleteByID(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}

	return r.db.DB.WithContext(ctx).Where("id = ?", id).Delete(&reportModel{}).Error
}

func toDomainReport(model reportModel) (domain.Report, error) {
	var data domain.ReportData
	if err := json.Unmarshal(model.Data, &data); err != nil {
		return domain.Report{}, fmt.Errorf("unmarshal report data: %w", err)
	}

	return domain.Report{
		ID:        model.ID,
		SessionID: model.SessionID,
		UserID:    model.UserID,
		CreatedAt: model.CreatedAt,
		Data:      data,
	}, nil
}

func migrateReports(tx *gorm.DB) error {
	return tx.AutoMigrate(&reportModel{})
}
