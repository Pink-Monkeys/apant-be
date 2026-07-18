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

func (r *MemoryScanRepository) FindByUserIDFiltered(_ context.Context, userID string, filter domain.ScanFilter) ([]domain.Scan, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]domain.Scan, 0)
	target := strings.TrimSpace(filter.Target)
	for _, scan := range r.scans {
		if userID != "" && scan.UserID != userID {
			continue
		}
		if !filter.From.IsZero() && scan.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && scan.CreatedAt.After(filter.To) {
			continue
		}
		if target != "" && scan.Target != target {
			continue
		}
		matched = append(matched, scan)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := int64(len(matched))

	if filter.Limit > 0 {
		start := 0
		if filter.Page > 1 {
			start = (filter.Page - 1) * filter.Limit
		}
		if start >= len(matched) {
			return []domain.Scan{}, total, nil
		}
		end := start + filter.Limit
		if end > len(matched) {
			end = len(matched)
		}
		matched = matched[start:end]
	}

	return matched, total, nil
}

func (r *MemoryScanRepository) DistinctTargets(_ context.Context, userID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	targets := make([]string, 0)
	for _, scan := range r.scans {
		if userID != "" && scan.UserID != userID {
			continue
		}
		t := strings.TrimSpace(scan.Target)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		targets = append(targets, t)
	}

	sort.Strings(targets)
	return targets, nil
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
