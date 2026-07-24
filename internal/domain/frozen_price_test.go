package domain

import "testing"

// TestFrozenPriceFreeVsUnknown locks the core distinction the pointer encoding
// exists for: a genuinely free model (priced 0.0) is NOT the same as an unpriced
// model (nil), even though both yield cost 0.
func TestFrozenPriceFreeVsUnknown(t *testing.T) {
	// Unknown: zero-value FrozenPrice (nil pointers) is not priced and refuses a cost.
	var unknown FrozenPrice
	if unknown.Priced() {
		t.Error("zero-value FrozenPrice must not be priced")
	}
	if cost, ok := unknown.Cost(1_000_000, 500_000); ok {
		t.Errorf("unpriced Cost must return ok=false, got cost=%v ok=true", cost)
	}

	// Free: explicitly priced at 0.0 — priced, and cost is a genuine zero.
	free := NewFrozenPrice(0, 0, "USD")
	if !free.Priced() {
		t.Error("a 0.0 price is genuinely priced (free), not unknown")
	}
	if cost, ok := free.Cost(1_000_000, 500_000); !ok || cost != 0 {
		t.Errorf("free Cost = (%v, %v), want (0, true)", cost, ok)
	}
}

func TestFrozenPriceCost(t *testing.T) {
	// 3.00 in / 15.00 out per 1M. 1M in + 0.5M out => 3.00 + 7.50 = 10.50.
	p := NewFrozenPrice(3, 15, "USD")
	cost, ok := p.Cost(1_000_000, 500_000)
	if !ok {
		t.Fatal("priced Cost must return ok=true")
	}
	if cost != 10.5 {
		t.Errorf("Cost = %v, want 10.5", cost)
	}
	if p.CurrencyOr("EUR") != "USD" {
		t.Errorf("CurrencyOr = %q, want USD", p.CurrencyOr("EUR"))
	}
}

// TestFrozenPriceHalfSet guards the atomic-snapshot invariant defensively: a
// snapshot with only one of In/Out is treated as not priced rather than computing
// a wrong half-cost.
func TestFrozenPriceHalfSet(t *testing.T) {
	in := 3.0
	half := FrozenPrice{In: &in}
	if half.Priced() {
		t.Error("half-set price (In only) must not be priced")
	}
	if _, ok := half.Cost(1_000_000, 0); ok {
		t.Error("half-set Cost must return ok=false")
	}
	if half.CurrencyOr("USD") != "USD" {
		t.Errorf("CurrencyOr with nil currency = %q, want USD", half.CurrencyOr("USD"))
	}
}
