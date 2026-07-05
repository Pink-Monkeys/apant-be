package db

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"apant_be/internal/domain"
)

type MemoryScanRepository struct {
	mu    sync.RWMutex
	scans map[string]domain.Scan
}

func NewMemoryScanRepository() *MemoryScanRepository {
	return &MemoryScanRepository{scans: make(map[string]domain.Scan)}
}

func (r *MemoryScanRepository) Save(_ context.Context, scan domain.Scan) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(scan.ID) == "" {
		return fmt.Errorf("scan id is required")
	}

	if scan.CreatedAt.IsZero() {
		scan.CreatedAt = time.Now()
	}

	r.scans[scan.ID] = scan
	return nil
}

func (r *MemoryScanRepository) FindByID(_ context.Context, id string) (domain.Scan, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scan, ok := r.scans[id]
	if !ok {
		return domain.Scan{}, fmt.Errorf("scan not found")
	}

	return scan, nil
}

func (r *MemoryScanRepository) FindByUserID(_ context.Context, userID string) ([]domain.Scan, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Scan, 0)
	for _, scan := range r.scans {
		if userID == "" || scan.UserID == userID {
			out = append(out, scan)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})

	return out, nil
}

func (r *MemoryScanRepository) FindRunningByUserID(_ context.Context, userID string) (domain.Scan, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var latest domain.Scan
	found := false
	for _, scan := range r.scans {
		if scan.UserID != userID || scan.Status != domain.ScanStatusRunning {
			continue
		}
		if !found || scan.CreatedAt.After(latest.CreatedAt) {
			latest = scan
			found = true
		}
	}
	return latest, found, nil
}

func (r *MemoryScanRepository) FailStaleRunning(_ context.Context, olderThan time.Time, reason string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var count int64
	for id, scan := range r.scans {
		if scan.Status == domain.ScanStatusRunning && scan.CreatedAt.Before(olderThan) {
			scan.Status = domain.ScanStatusFailed
			scan.Error = reason
			scan.UpdatedAt = time.Now()
			r.scans[id] = scan
			count++
		}
	}
	return count, nil
}
