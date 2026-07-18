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

func TestFindByUserIDFiltered(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryScanRepository()

	now := time.Now()
	_ = repo.Save(ctx, domain.Scan{ID: "a", UserID: "u1", Target: "http://x", CreatedAt: now.Add(-72 * time.Hour)})
	_ = repo.Save(ctx, domain.Scan{ID: "b", UserID: "u1", Target: "http://y", CreatedAt: now.Add(-1 * time.Hour)})
	_ = repo.Save(ctx, domain.Scan{ID: "c", UserID: "u1", Target: "http://x", CreatedAt: now})
	_ = repo.Save(ctx, domain.Scan{ID: "d", UserID: "u2", Target: "http://x", CreatedAt: now}) // other user

	// User scope + target filter: u1 scans on http://x -> a, c (newest first).
	got, total, err := repo.FindByUserIDFiltered(ctx, "u1", domain.ScanFilter{Target: "http://x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if total != 2 || len(got) != 2 || got[0].ID != "c" || got[1].ID != "a" {
		t.Fatalf("target filter wrong: total=%d ids=%v", total, ids(got))
	}

	// Time range: only scans in the last 24h -> b, c.
	got, total, _ = repo.FindByUserIDFiltered(ctx, "u1", domain.ScanFilter{From: now.Add(-24 * time.Hour)})
	if total != 2 || len(got) != 2 {
		t.Fatalf("time filter wrong: total=%d ids=%v", total, ids(got))
	}

	// Pagination: limit 1, page 2 of u1's 3 scans -> the 2nd newest (b).
	got, total, _ = repo.FindByUserIDFiltered(ctx, "u1", domain.ScanFilter{Page: 2, Limit: 1})
	if total != 3 || len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("pagination wrong: total=%d ids=%v", total, ids(got))
	}
}

func TestDistinctScanTargets(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryScanRepository()

	_ = repo.Save(ctx, domain.Scan{ID: "a", UserID: "u1", Target: "http://y"})
	_ = repo.Save(ctx, domain.Scan{ID: "b", UserID: "u1", Target: "http://x"})
	_ = repo.Save(ctx, domain.Scan{ID: "c", UserID: "u1", Target: "http://x"}) // dup
	_ = repo.Save(ctx, domain.Scan{ID: "d", UserID: "u1", Target: ""})         // empty skipped
	_ = repo.Save(ctx, domain.Scan{ID: "e", UserID: "u2", Target: "http://z"}) // other user

	targets, err := repo.DistinctTargets(ctx, "u1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(targets) != 2 || targets[0] != "http://x" || targets[1] != "http://y" {
		t.Fatalf("distinct targets wrong (want sorted [x y]): %v", targets)
	}
}

func ids(scans []domain.Scan) []string {
	out := make([]string, len(scans))
	for i, s := range scans {
		out[i] = s.ID
	}
	return out
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
