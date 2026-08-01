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
	ID        string `json:"id"`
	ScanID    string `json:"scan_id"`
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	// Username is a snapshot of the account that ran the scan, captured at report
	// creation. It stays correct even if the user is later renamed or deleted.
	// Empty for reports created before this field existed.
	Username  string     `json:"username,omitempty"`
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
	Mitigation       string              `json:"mitigation,omitempty"`
	Conclusion       string              `json:"conclusion"`
}

type ReportMetadata struct {
	Target      string    `json:"target"`
	Description string    `json:"description,omitempty"`
	ScanType    string    `json:"scan_type,omitempty"`
	ScanDate    time.Time `json:"scan_date"`
	Duration    string    `json:"duration"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	ToolsUsed   []string  `json:"tools_used"`
	TotalSteps  int       `json:"total_steps"`
}

type ReportTargetInfo struct {
	URL             string   `json:"url"`
	IPAddress       string   `json:"ip_address"`
	WebServer       string   `json:"web_server"`
	OperatingSystem string   `json:"operating_system"`
	TechStack       []string `json:"tech_stack"`
	OpenPorts       []string `json:"open_ports"`
	StatusCode      int      `json:"status_code"`
	Status          string   `json:"status"`
	CDN             string   `json:"cdn"`
	PageTitle       string   `json:"page_title"`
}

type ReportAttackSurface struct {
	SubdomainsFound        int `json:"subdomains_found"`
	URLsCrawled            int `json:"urls_crawled"`
	ParameterizedEndpoints int `json:"parameterized_endpoints"`
	OpenPortsCount         int `json:"open_ports_count"`
	// FilesAnalyzed is the number of source files examined in a static (SAST)
	// scan. Omitted for dynamic (DAST) scans.
	FilesAnalyzed int `json:"files_analyzed,omitempty"`
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
	// Status is the finding's state in the finder→validator pipeline: "validated"
	// (evidence in the scan transcript confirms it), "candidate" (reported but not
	// independently confirmed — e.g. an agent claim whose endpoint the scan never
	// exercised, or a flag-tier observation), or "rejected" (contradicted; dropped
	// before the report). Deterministic proof-tier detectors are validated; an agent's
	// self-reported "verified" is downgraded to candidate unless the transcript backs it.
	Status string `json:"status,omitempty"`
	// CVSSScore is a representative CVSS v3.1 base score for the finding's class
	// (the qualitative band it maps to). CWE is the matching Common Weakness
	// Enumeration id. Both are assigned deterministically per vulnerability class.
	CVSSScore float64 `json:"cvss_score,omitempty"`
	CWE       string  `json:"cwe,omitempty"`

	// CodeLocation is populated for static (SAST) findings: the precise file and
	// line span in the analyzed source tree. Empty for dynamic (DAST) findings.
	CodeLocation *ReportCodeLocation `json:"code_location,omitempty"`
}

// ReportCodeLocation pinpoints a SAST finding in the uploaded source tree. Paths
// are always relative to the repository root (never absolute server paths).
type ReportCodeLocation struct {
	FilePath    string `json:"file_path"`
	LineStart   int    `json:"line_start"`
	LineEnd     int    `json:"line_end,omitempty"`
	CodeSnippet string `json:"code_snippet,omitempty"`
	RuleID      string `json:"rule_id,omitempty"`
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
