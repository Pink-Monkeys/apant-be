package domain

import (
	"context"
	"time"
)

// ScanFilter narrows ListScans to a time range and/or target. Zero values mean
// "no constraint" (From/To zero = unbounded, Target empty = any target). Page is
// 1-indexed; Page/Limit of 0 mean "no pagination" (return everything). Mirrors
// ReportFilter so the scan list shares the report list's filter/pagination shape.
type ScanFilter struct {
	From   time.Time
	To     time.Time
	Target string
	Page   int
	Limit  int
}

// ScanRepository defines persistence contract for scan data.
type ScanRepository interface {
	Save(ctx context.Context, scan Scan) error
	FindByID(ctx context.Context, id string) (Scan, error)
	FindByUserID(ctx context.Context, userID string) ([]Scan, error)
	// FindByUserIDFiltered applies filter's time range/target/pagination on top of
	// the user scope and returns the matching page plus the total count of matching
	// rows (ignoring pagination), for the FE to compute page count.
	FindByUserIDFiltered(ctx context.Context, userID string, filter ScanFilter) ([]Scan, int64, error)
	// DistinctTargets returns the distinct, non-empty target values across the
	// user's scans, sorted ascending, for populating a target filter dropdown.
	DistinctTargets(ctx context.Context, userID string) ([]string, error)
	// FindRunningByUserID returns the user's currently-running scan, if any, to
	// enforce "one running scan per user".
	FindRunningByUserID(ctx context.Context, userID string) (Scan, bool, error)
	// FailStaleRunning marks scans stuck in "running" older than the cutoff as
	// failed. Called at boot to recover from a crash/restart mid-scan. Returns
	// the number of scans updated.
	FailStaleRunning(ctx context.Context, olderThan time.Time, reason string) (int64, error)
}

// SessionRepository defines persistence contract for session timelines.
type SessionRepository interface {
	Create() *Session
	Get(id string) (*Session, bool)
	AddMessage(id string, msg Message) (*Session, error)
	List() []*Session
}
