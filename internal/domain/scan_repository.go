package domain

import (
	"context"
	"time"
)

// ScanRepository defines persistence contract for scan data.
type ScanRepository interface {
	Save(ctx context.Context, scan Scan) error
	FindByID(ctx context.Context, id string) (Scan, error)
	FindByUserID(ctx context.Context, userID string) ([]Scan, error)
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
