package scanner

import "apant_be/internal/domain"

type ToolRegistry struct {
	tools []domain.ToolInfo
}

func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: []domain.ToolInfo{
			{Name: "nmap_scan", Description: "Run Nmap scan via Docker executor", Enabled: true},
			{Name: "sqlmap_scan", Description: "Planned SQLMap scan via Docker executor", Enabled: false},
			{Name: "nikto_scan", Description: "Planned Nikto web scan via Docker executor", Enabled: false},
		},
	}
}

func (r *ToolRegistry) List() []domain.ToolInfo {
	return r.tools
}
