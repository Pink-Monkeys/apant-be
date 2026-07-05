package db

import (
	"context"
	"testing"
	"time"

	"apant_be/internal/domain"
)

func TestFindRunningByUserID(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryScanRepository()

	// user u1: one completed, one running (running is the latest).
	_ = repo.Save(ctx, domain.Scan{ID: "s1", UserID: "u1", Status: domain.ScanStatusCompleted, CreatedAt: time.Now().Add(-time.Hour)})
	_ = repo.Save(ctx, domain.Scan{ID: "s2", UserID: "u1", Status: domain.ScanStatusRunning, CreatedAt: time.Now()})
	// user u2: only completed.
	_ = repo.Save(ctx, domain.Scan{ID: "s3", UserID: "u2", Status: domain.ScanStatusCompleted, CreatedAt: time.Now()})

	got, found, err := repo.FindRunningByUserID(ctx, "u1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !found || got.ID != "s2" {
		t.Fatalf("expected running scan s2, got found=%v id=%s", found, got.ID)
	}

	if _, found, _ := repo.FindRunningByUserID(ctx, "u2"); found {
		t.Fatal("u2 has no running scan")
	}
}

func TestFailStaleRunning(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryScanRepository()

	old := time.Now().Add(-time.Hour)
	fresh := time.Now()
	_ = repo.Save(ctx, domain.Scan{ID: "stale", UserID: "u1", Status: domain.ScanStatusRunning, CreatedAt: old})
	_ = repo.Save(ctx, domain.Scan{ID: "fresh", UserID: "u1", Status: domain.ScanStatusRunning, CreatedAt: fresh})
	_ = repo.Save(ctx, domain.Scan{ID: "done", UserID: "u1", Status: domain.ScanStatusCompleted, CreatedAt: old})

	cutoff := time.Now().Add(-30 * time.Minute)
	n, err := repo.FailStaleRunning(ctx, cutoff, "interrupted")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 stale scan failed, got %d", n)
	}

	stale, _ := repo.FindByID(ctx, "stale")
	if stale.Status != domain.ScanStatusFailed || stale.Error != "interrupted" {
		t.Fatalf("stale scan not failed: status=%s error=%q", stale.Status, stale.Error)
	}
	freshScan, _ := repo.FindByID(ctx, "fresh")
	if freshScan.Status != domain.ScanStatusRunning {
		t.Fatal("fresh running scan must be untouched")
	}
}
