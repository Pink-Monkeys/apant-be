package domain

import "context"

// ScanRepository defines persistence contract for scan data.
type ScanRepository interface {
	Save(ctx context.Context, scan Scan) error
	FindByID(ctx context.Context, id string) (Scan, error)
}

// SessionRepository defines persistence contract for session timelines.
type SessionRepository interface {
	Create() *Session
	Get(id string) (*Session, bool)
	AddMessage(id string, msg Message) (*Session, error)
	List() []*Session
}
