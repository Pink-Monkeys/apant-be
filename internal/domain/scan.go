package domain

import "time"

// Scan is the core business entity for a pentest run.
type Scan struct {
	ID        string
	Target    string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
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
