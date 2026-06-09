package domain

import "time"

// Scan is the core business entity for a pentest run. It persists the raw
// execution trace (steps) so reports can be generated and regenerated from it.
type Scan struct {
	ID          string
	SessionID   string
	UserID      string
	Target      string
	Provider    string
	Model       string
	Message     string
	Status      string
	Steps       []ScanStep
	FinalAnswer string
	Duration    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
