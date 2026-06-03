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

func (r *MemoryReportRepository) DeleteByID(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.reports, id)
	return nil
}
