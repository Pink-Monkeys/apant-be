package domain

import "time"

// Scan lifecycle statuses. A scan starts "running" (async) and ends in one of
// the terminal states. The legacy synchronous path writes "completed" directly.
const (
	ScanStatusRunning   = "running"
	ScanStatusCompleted = "completed"
	ScanStatusFailed    = "failed"
	ScanStatusCancelled = "cancelled"
)

// Scan is the core business entity for a pentest run. It persists the raw
// execution trace (steps) so reports can be generated and regenerated from it.
type Scan struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	// Username is a snapshot of the account that started the scan, captured when
	// the scan is created. It stays correct even if the user is later renamed or
	// deleted. Empty for scans created before this field existed.
	Username    string      `json:"username,omitempty"`
	Target      string      `json:"target"`
	TargetInfo  *TargetInfo `json:"target_info,omitempty"`
	Provider    string      `json:"provider"`
	Model       string      `json:"model"`
	Message     string      `json:"message"`
	Description string      `json:"description,omitempty"`
	ScanType    string      `json:"scan_type,omitempty"`
	Status      string      `json:"status"`
	Steps       []ScanStep  `json:"steps"`
	FinalAnswer string      `json:"final_answer"`
	// Error holds a human-readable reason when Status is "failed" (e.g. a
	// rate-limit message from the provider), for the FE to display.
	Error string `json:"error,omitempty"`
	// LLM token usage for the run, accumulated across every model call (login
	// prelude, each step, final synthesis) and frozen at the terminal save. All
	// zero when the scan made no model calls; Calls>0 with TotalTokens==0 means
	// the provider reported no usage. TotalTokens is the provider's own total
	// when reported, else InputTokens+OutputTokens.
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
	Calls        int `json:"calls,omitempty"`
	// Price snapshot frozen at scan start: the per-1M-token price of the model in
	// effect when the scan began, immune to later admin edits. nil pointers mean
	// the model had no price configured (cost is unknowable for this run); a
	// non-nil 0.0 means a genuinely free model. Set/read atomically via
	// SetPrice/FrozenPrice.
	PriceInPer1M  *float64 `json:"price_in_per_1m,omitempty"`
	PriceOutPer1M *float64 `json:"price_out_per_1m,omitempty"`
	PriceCurrency *string  `json:"price_currency,omitempty"`
	// Cost is a DERIVED view (not persisted): it is computed from the frozen price
	// and token counts at display time via CostBreakdown. Populated by the read
	// path; nil/omitted otherwise.
	Cost      *CostInfo `json:"cost,omitempty"`
	Duration  string    `json:"duration"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Cost states. These distinguish the *reasons* a cost may be absent, so analysis
// can exclude runs consciously rather than treating every gap as zero:
//   - CostComputed:  price known and usage reported; Amount is set (may be 0 for a
//     genuinely free model).
//   - CostUnpriced:  usage reported but the model had no price configured at scan
//     start; the cost is unknowable for this run.
//   - CostNoUsage:   the provider reported no token usage (calls>0, tokens==0).
//   - CostNoCalls:   the scan made no model calls at all (e.g. failed early).
const (
	CostComputed = "computed"
	CostUnpriced = "unpriced"
	CostNoUsage  = "no_usage"
	CostNoCalls  = "no_calls"
)

// CostInfo is the derived cost view for a scan. Amount is set only when State is
// CostComputed.
type CostInfo struct {
	State    string   `json:"state"`
	Amount   *float64 `json:"amount,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

// CostBreakdown derives the scan's cost from its frozen price and token counts.
// The two axes are kept independent: whether the price is known (FrozenPrice) and
// whether usage was reported (derived from the persisted counts — a real call
// never legitimately consumes zero input tokens, so calls>0 with zero total means
// the provider stayed silent). Pure: no side effects, safe to call on any scan.
func (s Scan) CostBreakdown() CostInfo {
	price := s.FrozenPrice()
	switch {
	case s.Calls == 0:
		return CostInfo{State: CostNoCalls}
	case s.TotalTokens == 0:
		// calls>0 but nothing reported: usage axis failed.
		return CostInfo{State: CostNoUsage}
	case !price.Priced():
		// usage reported but price axis unknown.
		return CostInfo{State: CostUnpriced}
	default:
		amount, _ := price.Cost(s.InputTokens, s.OutputTokens)
		return CostInfo{State: CostComputed, Amount: &amount, Currency: price.CurrencyOr("USD")}
	}
}

// SetPrice stores a frozen price snapshot atomically (all three fields together),
// so the scan never carries a half-set price.
func (s *Scan) SetPrice(p FrozenPrice) {
	s.PriceInPer1M = p.In
	s.PriceOutPer1M = p.Out
	s.PriceCurrency = p.Currency
}

// FrozenPrice reconstructs the scan's price snapshot for cost derivation.
func (s Scan) FrozenPrice() FrozenPrice {
	return FrozenPrice{In: s.PriceInPer1M, Out: s.PriceOutPer1M, Currency: s.PriceCurrency}
}

// TargetInfo is a deterministic fingerprint of the scan target for the FE
// "Target Information" panel. OperatingSystem is best-effort.
type TargetInfo struct {
	Address         string   `json:"address"`
	Server          string   `json:"server,omitempty"`
	OperatingSystem string   `json:"operating_system,omitempty"`
	Technologies    []string `json:"technologies,omitempty"`
	Status          string   `json:"status,omitempty"`
	CDN             string   `json:"cdn,omitempty"`
	IP              string   `json:"ip,omitempty"`
	Title           string   `json:"title,omitempty"`
	StatusCode      int      `json:"status_code,omitempty"`
}

// ScanStep is a single tool execution within a scan run.
type ScanStep struct {
	StepNumber int            `json:"step"`
	ToolName   string         `json:"tool"`
	ToolParams map[string]any `json:"params"`
	Result     map[string]any `json:"result"`
	Summary    string         `json:"summary"`
}

// Message captures agent conversation and tool execution traces in a session.
type Message struct {
	Role      string
	Type      string
	Content   string
	ToolName  string
	ToolInput map[string]any
	Result    map[string]any
	CreatedAt time.Time
}

// Session stores the conversation timeline for an agent interaction.
type Session struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	Messages  []Message
}
