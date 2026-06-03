package domain

import "context"

// ReportRepository defines persistence contract for reports.
type ReportRepository interface {
	Save(ctx context.Context, report Report) error
	FindByID(ctx context.Context, id string) (Report, error)
	FindByUserID(ctx context.Context, userID string) ([]Report, error)
	DeleteByID(ctx context.Context, id string) error
}
