package domain

import "testing"

func f64(v float64) *float64 { return &v }

// TestCostBreakdown locks the two-axis derivation: whether the price is known
// (FrozenPrice) and whether usage was reported (token counts), and the four
// distinguishable states that result — including the crucial free-vs-unknown and
// no-usage-vs-unpriced separations.
func TestCostBreakdown(t *testing.T) {
	usd := "USD"
	cases := []struct {
		name       string
		scan       Scan
		wantState  string
		wantAmount *float64
	}{
		{
			name:       "computed: priced + usage reported",
			scan:       Scan{Calls: 3, InputTokens: 1_000_000, OutputTokens: 1_000_000, TotalTokens: 2_000_000, PriceInPer1M: f64(3), PriceOutPer1M: f64(15), PriceCurrency: &usd},
			wantState:  CostComputed,
			wantAmount: f64(18), // 3 + 15
		},
		{
			name:       "computed free: priced 0 + usage reported => genuine zero",
			scan:       Scan{Calls: 2, InputTokens: 500_000, OutputTokens: 500_000, TotalTokens: 1_000_000, PriceInPer1M: f64(0), PriceOutPer1M: f64(0), PriceCurrency: &usd},
			wantState:  CostComputed,
			wantAmount: f64(0),
		},
		{
			name:      "unpriced: usage reported but no price configured",
			scan:      Scan{Calls: 2, InputTokens: 500_000, OutputTokens: 200_000, TotalTokens: 700_000},
			wantState: CostUnpriced,
		},
		{
			name:      "no_usage: calls made but provider reported nothing",
			scan:      Scan{Calls: 4, InputTokens: 0, OutputTokens: 0, TotalTokens: 0, PriceInPer1M: f64(3), PriceOutPer1M: f64(15)},
			wantState: CostNoUsage,
		},
		{
			name:      "no_calls: scan made no model calls",
			scan:      Scan{Calls: 0},
			wantState: CostNoCalls,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.scan.CostBreakdown()
			if got.State != c.wantState {
				t.Fatalf("state = %q, want %q", got.State, c.wantState)
			}
			switch {
			case c.wantAmount == nil && got.Amount != nil:
				t.Errorf("amount = %v, want nil", *got.Amount)
			case c.wantAmount != nil && got.Amount == nil:
				t.Errorf("amount = nil, want %v", *c.wantAmount)
			case c.wantAmount != nil && *got.Amount != *c.wantAmount:
				t.Errorf("amount = %v, want %v", *got.Amount, *c.wantAmount)
			}
			if c.wantState == CostComputed && got.Currency != "USD" {
				t.Errorf("currency = %q, want USD", got.Currency)
			}
		})
	}
}
