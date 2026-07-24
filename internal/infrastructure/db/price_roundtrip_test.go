package db

import (
	"context"
	"testing"

	"apant_be/internal/domain"
)

func fptr(v float64) *float64 { return &v }

// TestLLMModelPriceRoundTripNilNotZero is the "prove it once" guardrail for the
// free-vs-unknown distinction at the persistence-mapping boundary: an unpriced
// model (nil) must survive fromDomain->toDomain as nil, and a genuinely free model
// (0.0) must survive as a non-nil 0.0. If a future change coerces nil into a
// float64 zero (the footgun the pointer encoding exists to prevent), this fails.
func TestLLMModelPriceRoundTripNilNotZero(t *testing.T) {
	t.Run("unpriced stays nil", func(t *testing.T) {
		got := toDomainLLMModel(fromDomainLLMModel(domain.LLMModel{ID: "m1", ProviderID: "p1", ModelID: "x"}))
		if got.PriceInPer1M != nil || got.PriceOutPer1M != nil || got.Currency != nil {
			t.Fatalf("unpriced model must round-trip as nil, got in=%v out=%v cur=%v",
				got.PriceInPer1M, got.PriceOutPer1M, got.Currency)
		}
		if got.FrozenPrice().Priced() {
			t.Error("round-tripped unpriced model must not be Priced()")
		}
	})

	t.Run("free (0.0) stays priced-zero", func(t *testing.T) {
		usd := "USD"
		got := toDomainLLMModel(fromDomainLLMModel(domain.LLMModel{
			ID: "m2", ProviderID: "p1", ModelID: "free",
			PriceInPer1M: fptr(0), PriceOutPer1M: fptr(0), Currency: &usd,
		}))
		if got.PriceInPer1M == nil || got.PriceOutPer1M == nil {
			t.Fatal("free model must round-trip as non-nil 0.0, not nil")
		}
		if !got.FrozenPrice().Priced() {
			t.Error("free model (0.0) must be Priced(), distinct from unpriced")
		}
		if *got.PriceInPer1M != 0 || *got.PriceOutPer1M != 0 {
			t.Errorf("free prices must stay 0, got in=%v out=%v", *got.PriceInPer1M, *got.PriceOutPer1M)
		}
	})

	t.Run("real price stays exact", func(t *testing.T) {
		usd := "USD"
		got := toDomainLLMModel(fromDomainLLMModel(domain.LLMModel{
			ID: "m3", ProviderID: "p1", ModelID: "paid",
			PriceInPer1M: fptr(3), PriceOutPer1M: fptr(15), Currency: &usd,
		}))
		if got.PriceInPer1M == nil || *got.PriceInPer1M != 3 || got.PriceOutPer1M == nil || *got.PriceOutPer1M != 15 {
			t.Fatalf("price must round-trip exactly, got in=%v out=%v", got.PriceInPer1M, got.PriceOutPer1M)
		}
	})
}

// TestScanPriceRoundTripNilNotZero locks the same distinction across a real
// write->read on the in-memory scan repository (the memory backend stores the
// domain struct verbatim, so this proves the domain field carries nil vs 0.0
// through a persistence round-trip).
func TestScanPriceRoundTripNilNotZero(t *testing.T) {
	repo := NewMemoryScanRepository()
	ctx := context.Background()

	unpriced := domain.Scan{ID: "s-unpriced", Status: domain.ScanStatusCompleted, Calls: 2, TotalTokens: 100}
	free := domain.Scan{ID: "s-free", Status: domain.ScanStatusCompleted, Calls: 2, TotalTokens: 100}
	free.SetPrice(domain.NewFrozenPrice(0, 0, "USD"))

	for _, s := range []domain.Scan{unpriced, free} {
		if err := repo.Save(ctx, s); err != nil {
			t.Fatalf("save %s: %v", s.ID, err)
		}
	}

	gotUnpriced, err := repo.FindByID(ctx, "s-unpriced")
	if err != nil {
		t.Fatal(err)
	}
	if gotUnpriced.FrozenPrice().Priced() {
		t.Error("unpriced scan must round-trip as unpriced, not priced-zero")
	}
	if gotUnpriced.CostBreakdown().State != domain.CostUnpriced {
		t.Errorf("unpriced scan cost state = %q, want %q", gotUnpriced.CostBreakdown().State, domain.CostUnpriced)
	}

	gotFree, err := repo.FindByID(ctx, "s-free")
	if err != nil {
		t.Fatal(err)
	}
	if !gotFree.FrozenPrice().Priced() {
		t.Error("free scan (0.0) must round-trip as priced, distinct from unpriced")
	}
	cb := gotFree.CostBreakdown()
	if cb.State != domain.CostComputed || cb.Amount == nil || *cb.Amount != 0 {
		t.Errorf("free scan cost = %+v, want computed zero", cb)
	}
}
