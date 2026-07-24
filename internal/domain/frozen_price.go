package domain

// FrozenPrice is the per-1M-token price captured at the moment a scan begins and
// stored, immutable, on the scan record. It is the *one* explicitly-encoded axis
// of cost state: whether the price is known.
//
// The pointer fields exist precisely so that "price not set" (nil) never collides
// with "price is genuinely zero" (a non-nil 0.0, e.g. a free/local model). A plain
// float64 zero value would conflate free with unknown — the exact bug this type
// prevents at the type level. In, Out and Currency are an atomic snapshot: they
// are set together or all nil.
//
// The other axis — whether token usage was reported — is NOT stored here; it is
// derived from the persisted token counts (Calls>0 && TotalTokens==0 => the
// provider reported no usage), because a real model call can never legitimately
// consume zero input tokens.
//
// Access is gated: callers obtain a cost through Cost, which refuses to return a
// number unless the price is actually known. There is no supported path that
// reads In/Out without passing the Priced() check.
type FrozenPrice struct {
	In       *float64 // input price per 1M tokens
	Out      *float64 // output price per 1M tokens
	Currency *string  // e.g. "USD"; informational, defaults via CurrencyOr
}

// NewFrozenPrice builds a priced snapshot from concrete values. Currency is
// normalized to "USD" when empty so a stored price always carries a currency.
func NewFrozenPrice(in, out float64, currency string) FrozenPrice {
	if currency == "" {
		currency = "USD"
	}
	return FrozenPrice{In: &in, Out: &out, Currency: &currency}
}

// Priced reports whether the price is known. A half-set snapshot (only one of
// In/Out) is treated as not priced, since a cost cannot be computed from it.
func (p FrozenPrice) Priced() bool {
	return p.In != nil && p.Out != nil
}

// Cost returns the run cost for the given token counts and true, or (0, false)
// when the price is not known. A priced-but-zero snapshot (free model) returns
// (0, true) — a genuine zero cost, distinct from the unknown case.
func (p FrozenPrice) Cost(inTokens, outTokens int) (float64, bool) {
	if !p.Priced() {
		return 0, false
	}
	cost := float64(inTokens)/1e6*(*p.In) + float64(outTokens)/1e6*(*p.Out)
	return cost, true
}

// CurrencyOr returns the snapshot's currency, or def when unset.
func (p FrozenPrice) CurrencyOr(def string) string {
	if p.Currency == nil || *p.Currency == "" {
		return def
	}
	return *p.Currency
}
