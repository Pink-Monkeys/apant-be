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
	ID          string      `json:"id"`
	SessionID   string      `json:"session_id"`
	UserID      string      `json:"user_id"`
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
	Error     string    `json:"error,omitempty"`
	Duration  string    `json:"duration"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
