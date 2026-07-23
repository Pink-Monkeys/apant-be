package scanner

import "apant_be/internal/domain"

type ToolRegistry struct {
	tools []domain.ToolInfo
}

func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: []domain.ToolInfo{
			{Name: "nmap_scan", Description: "Port scanning and service detection", Enabled: true},
			{Name: "wafw00f_detect", Description: "Fingerprint the Web Application Firewall (WAF) fronting the target (advisory, not a vulnerability)", Enabled: true},
			{Name: "httpx_probe", Description: "HTTP probing, status code, title, and technology detection", Enabled: true},
			{Name: "subfinder_enum", Description: "Passive subdomain enumeration", Enabled: true},
			{Name: "katana_crawl", Description: "Web crawler for URL discovery", Enabled: true},
			{Name: "gau_urls", Description: "Fetch known URLs from Wayback Machine, OTX, URLScan", Enabled: true},
			{Name: "waybackurls_fetch", Description: "Fetch URLs from Wayback Machine archive", Enabled: true},
			{Name: "ffuf_fuzz", Description: "Web fuzzing for directories and parameters", Enabled: true},
			{Name: "nuclei_scan", Description: "Vulnerability scanning with templates", Enabled: true},
			{Name: "dalfox_xss", Description: "XSS vulnerability scanning", Enabled: true},
			{Name: "sqlmap_scan", Description: "SQL injection detection and exploitation", Enabled: true},
			{Name: "http_request", Description: "Send custom HTTP request with custom method, headers, body, and cookies", Enabled: true},
			{Name: "mitmdump_intercept", Description: "HTTP/HTTPS traffic interception and analysis", Enabled: false},
			{Name: "semgrep_scan", Description: "Static analysis (SAST) over an uploaded source tree using Semgrep rulesets", Enabled: true},
			{Name: "gitleaks_scan", Description: "Detect hardcoded secrets and credentials in source code", Enabled: true},
			{Name: "osv_scan", Description: "Software composition analysis: known CVEs in declared dependencies", Enabled: true},
			{Name: "list_files", Description: "List files in the uploaded source tree", Enabled: true},
			{Name: "read_file", Description: "Read a file from the uploaded source tree (optionally a line range)", Enabled: true},
			{Name: "grep_code", Description: "Search the uploaded source tree for a regex pattern", Enabled: true},
		},
	}
}

func (r *ToolRegistry) List() []domain.ToolInfo {
	return r.tools
}
