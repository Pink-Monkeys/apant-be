package domain

import (
	"context"
	"time"
)

// ReportFilter narrows ListReports to a time range and/or target. Zero values
// mean "no constraint" (From/To zero = unbounded, Target empty = any target).
// Page is 1-indexed; Page/Limit of 0 mean "no pagination" (return everything).
type ReportFilter struct {
	From   time.Time
	To     time.Time
	Target string
	Page   int
	Limit  int
}

// ReportRepository defines persistence contract for reports.
type ReportRepository interface {
	Save(ctx context.Context, report Report) error
	FindByID(ctx context.Context, id string) (Report, error)
	FindByUserID(ctx context.Context, userID string) ([]Report, error)
	// FindByUserIDFiltered applies filter's time range/target/pagination on top of
	// the user scope and returns the matching page plus the total count of
	// matching rows (ignoring pagination), for the FE to compute page count.
	FindByUserIDFiltered(ctx context.Context, userID string, filter ReportFilter) ([]Report, int64, error)
	// DistinctTargets returns the distinct, non-empty target values across the
	// user's reports, sorted ascending, for populating a target filter dropdown.
	DistinctTargets(ctx context.Context, userID string) ([]string, error)
	DeleteByID(ctx context.Context, id string) error
}
