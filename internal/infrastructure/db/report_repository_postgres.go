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
	ID               string    `gorm:"column:id;type:text;primaryKey"`
	ScanID           string    `gorm:"column:scan_id;type:text;index"`
	SessionID        string    `gorm:"column:session_id;type:text;index"`
	UserID           string    `gorm:"column:user_id;type:text;index"`
	Username         string    `gorm:"column:username;type:text"`
	Title            string    `gorm:"column:title;type:text"`
	OverallSeverity  string    `gorm:"column:overall_severity;type:text"`
	ExecutiveSummary string    `gorm:"column:executive_summary;type:text"`
	Mitigation       string    `gorm:"column:mitigation;type:text"`
	Conclusion       string    `gorm:"column:conclusion;type:text"`
	Metadata         []byte    `gorm:"column:metadata;type:jsonb"`
	TargetInfo       []byte    `gorm:"column:target_info;type:jsonb"`
	AttackSurface    []byte    `gorm:"column:attack_surface;type:jsonb"`
	Vulnerabilities  []byte    `gorm:"column:vulnerabilities;type:jsonb"`
	Statistics       []byte    `gorm:"column:statistics;type:jsonb"`
	CreatedAt        time.Time `gorm:"column:created_at;not null"`
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

	metadata, err := json.Marshal(report.Data.Metadata)
	if err != nil {
		return fmt.Errorf("marshal report metadata: %w", err)
	}
	targetInfo, err := json.Marshal(report.Data.TargetInfo)
	if err != nil {
		return fmt.Errorf("marshal report target info: %w", err)
	}
	attackSurface, err := json.Marshal(report.Data.AttackSurface)
	if err != nil {
		return fmt.Errorf("marshal report attack surface: %w", err)
	}
	vulnerabilities, err := json.Marshal(report.Data.Vulnerabilities)
	if err != nil {
		return fmt.Errorf("marshal report vulnerabilities: %w", err)
	}
	statistics, err := json.Marshal(report.Data.Statistics)
	if err != nil {
		return fmt.Errorf("marshal report statistics: %w", err)
	}

	model := reportModel{
		ID:               report.ID,
		ScanID:           strings.TrimSpace(report.ScanID),
		SessionID:        strings.TrimSpace(report.SessionID),
		UserID:           strings.TrimSpace(report.UserID),
		Username:         strings.TrimSpace(report.Username),
		Title:            report.Data.Title,
		OverallSeverity:  string(report.Data.OverallSeverity),
		ExecutiveSummary: report.Data.ExecutiveSummary,
		Mitigation:       report.Data.Mitigation,
		Conclusion:       report.Data.Conclusion,
		Metadata:         metadata,
		TargetInfo:       targetInfo,
		AttackSurface:    attackSurface,
		Vulnerabilities:  vulnerabilities,
		Statistics:       statistics,
		CreatedAt:        report.CreatedAt,
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
	data := domain.ReportData{
		Title:            model.Title,
		OverallSeverity:  domain.ReportSeverity(model.OverallSeverity),
		ExecutiveSummary: model.ExecutiveSummary,
		Mitigation:       model.Mitigation,
		Conclusion:       model.Conclusion,
	}

	if len(model.Metadata) > 0 {
		if err := json.Unmarshal(model.Metadata, &data.Metadata); err != nil {
			return domain.Report{}, fmt.Errorf("unmarshal report metadata: %w", err)
		}
	}
	if len(model.TargetInfo) > 0 {
		if err := json.Unmarshal(model.TargetInfo, &data.TargetInfo); err != nil {
			return domain.Report{}, fmt.Errorf("unmarshal report target info: %w", err)
		}
	}
	if len(model.AttackSurface) > 0 {
		if err := json.Unmarshal(model.AttackSurface, &data.AttackSurface); err != nil {
			return domain.Report{}, fmt.Errorf("unmarshal report attack surface: %w", err)
		}
	}
	if len(model.Vulnerabilities) > 0 {
		if err := json.Unmarshal(model.Vulnerabilities, &data.Vulnerabilities); err != nil {
			return domain.Report{}, fmt.Errorf("unmarshal report vulnerabilities: %w", err)
		}
	}
	if len(model.Statistics) > 0 {
		if err := json.Unmarshal(model.Statistics, &data.Statistics); err != nil {
			return domain.Report{}, fmt.Errorf("unmarshal report statistics: %w", err)
		}
	}

	return domain.Report{
		ID:        model.ID,
		ScanID:    model.ScanID,
		SessionID: model.SessionID,
		UserID:    model.UserID,
		Username:  model.Username,
		CreatedAt: model.CreatedAt,
		Data:      data,
	}, nil
}

func migrateReports(tx *gorm.DB) error {
	return tx.AutoMigrate(&reportModel{})
}
