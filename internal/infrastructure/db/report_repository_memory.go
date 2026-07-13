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

type MemoryReportRepository struct {
	mu      sync.RWMutex
	reports map[string]domain.Report
}

func NewMemoryReportRepository() *MemoryReportRepository {
	return &MemoryReportRepository{reports: make(map[string]domain.Report)}
}

func (r *MemoryReportRepository) Save(_ context.Context, report domain.Report) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if strings.TrimSpace(report.ID) == "" {
		return fmt.Errorf("report id is required")
	}

	if report.CreatedAt.IsZero() {
		report.CreatedAt = time.Now()
	}

	r.reports[report.ID] = report
	return nil
}

func (r *MemoryReportRepository) FindByID(_ context.Context, id string) (domain.Report, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report, ok := r.reports[id]
	if !ok {
		return domain.Report{}, fmt.Errorf("report not found")
	}

	return report, nil
}

func (r *MemoryReportRepository) FindByUserID(_ context.Context, userID string) ([]domain.Report, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]domain.Report, 0)
	for _, report := range r.reports {
		if userID == "" || report.UserID == userID {
			out = append(out, report)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})

	return out, nil
}

// FindByUserIDFiltered applies filter.From/To/Target/Page/Limit in-memory,
// mirroring the Postgres implementation's semantics.
func (r *MemoryReportRepository) FindByUserIDFiltered(_ context.Context, userID string, filter domain.ReportFilter) ([]domain.Report, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]domain.Report, 0)
	for _, report := range r.reports {
		if userID != "" && report.UserID != userID {
			continue
		}
		if !filter.From.IsZero() && report.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && report.CreatedAt.After(filter.To) {
			continue
		}
		if target := strings.TrimSpace(filter.Target); target != "" && report.Data.Metadata.Target != target {
			continue
		}
		matched = append(matched, report)
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	total := int64(len(matched))
	if filter.Limit <= 0 {
		return matched, total, nil
	}

	start := 0
	if filter.Page > 1 {
		start = (filter.Page - 1) * filter.Limit
	}
	if start >= len(matched) {
		return []domain.Report{}, total, nil
	}
	end := start + filter.Limit
	if end > len(matched) {
		end = len(matched)
	}

	return matched[start:end], total, nil
}

// DistinctTargets returns the distinct, non-empty target values across the
// user's reports, sorted ascending.
func (r *MemoryReportRepository) DistinctTargets(_ context.Context, userID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	for _, report := range r.reports {
		if userID != "" && report.UserID != userID {
			continue
		}
		target := strings.TrimSpace(report.Data.Metadata.Target)
		if target != "" {
			seen[target] = true
		}
	}

	out := make([]string, 0, len(seen))
	for target := range seen {
		out = append(out, target)
	}
	sort.Strings(out)

	return out, nil
}

func (r *MemoryReportRepository) DeleteByID(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.reports, id)
	return nil
}
