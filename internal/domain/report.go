package domain

import "time"

type ReportSeverity string

const (
	SeverityCritical      ReportSeverity = "critical"
	SeverityHigh          ReportSeverity = "high"
	SeverityMedium        ReportSeverity = "medium"
	SeverityLow           ReportSeverity = "low"
	SeverityInformational ReportSeverity = "informational"
)

type Report struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	UserID    string     `json:"user_id"`
	CreatedAt time.Time  `json:"created_at"`
	Data      ReportData `json:"data"`
}

type ReportData struct {
	Title            string              `json:"title"`
	OverallSeverity  ReportSeverity      `json:"overall_severity"`
	Metadata         ReportMetadata      `json:"metadata"`
	ExecutiveSummary string              `json:"executive_summary"`
	TargetInfo       ReportTargetInfo    `json:"target_info"`
	AttackSurface    ReportAttackSurface `json:"attack_surface"`
	Vulnerabilities  []ReportVuln        `json:"vulnerabilities"`
	Statistics       ReportStats         `json:"statistics"`
	Conclusion       string              `json:"conclusion"`
}

type ReportMetadata struct {
	Target     string    `json:"target"`
	ScanDate   time.Time `json:"scan_date"`
	Duration   string    `json:"duration"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	ToolsUsed  []string  `json:"tools_used"`
	TotalSteps int       `json:"total_steps"`
}

type ReportTargetInfo struct {
	URL        string   `json:"url"`
	IPAddress  string   `json:"ip_address"`
	WebServer  string   `json:"web_server"`
	TechStack  []string `json:"tech_stack"`
	OpenPorts  []string `json:"open_ports"`
	StatusCode int      `json:"status_code"`
	PageTitle  string   `json:"page_title"`
}

type ReportAttackSurface struct {
	SubdomainsFound        int `json:"subdomains_found"`
	URLsCrawled            int `json:"urls_crawled"`
	ParameterizedEndpoints int `json:"parameterized_endpoints"`
	OpenPortsCount         int `json:"open_ports_count"`
}

type ReportVuln struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Severity       ReportSeverity `json:"severity"`
	Type           string         `json:"type"`
	Location       string         `json:"location"`
	Description    string         `json:"description"`
	Impact         string         `json:"impact"`
	PoC            ReportPoC      `json:"poc"`
	Recommendation string         `json:"recommendation"`
	Verified       bool           `json:"verified"`
	CVSSScore      float64        `json:"cvss_score,omitempty"`
}

type ReportPoC struct {
	Method   string            `json:"method"`
	URL      string            `json:"url"`
	Payload  string            `json:"payload"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	Response string            `json:"response,omitempty"`
	CurlCmd  string            `json:"curl_cmd"`
}

type ReportStats struct {
	TotalVulnerabilities int            `json:"total_vulnerabilities"`
	BySeverity           map[string]int `json:"by_severity"`
	ExploitSuccessCount  int            `json:"exploit_success_count"`
	ScanStatus           string         `json:"scan_status"`
}
