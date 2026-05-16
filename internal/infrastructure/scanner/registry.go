package scanner

import "apant_be/internal/domain"

type ToolRegistry struct {
	tools []domain.ToolInfo
}

func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: []domain.ToolInfo{
			{Name: "nmap_scan", Description: "Port scanning and service detection", Enabled: true},
			{Name: "httpx_probe", Description: "HTTP probing, status code, title, and technology detection", Enabled: true},
			{Name: "subfinder_enum", Description: "Passive subdomain enumeration", Enabled: true},
			{Name: "katana_crawl", Description: "Web crawler for URL discovery", Enabled: true},
			{Name: "gau_urls", Description: "Fetch known URLs from Wayback Machine, OTX, URLScan", Enabled: true},
			{Name: "waybackurls_fetch", Description: "Fetch URLs from Wayback Machine archive", Enabled: true},
			{Name: "ffuf_fuzz", Description: "Web fuzzing for directories and parameters", Enabled: true},
			{Name: "nuclei_scan", Description: "Vulnerability scanning with templates", Enabled: true},
			{Name: "dalfox_xss", Description: "XSS vulnerability scanning", Enabled: true},
			{Name: "sqlmap_scan", Description: "SQL injection detection and exploitation", Enabled: true},
			{Name: "mitmdump_intercept", Description: "HTTP/HTTPS traffic interception and analysis", Enabled: false},
		},
	}
}

func (r *ToolRegistry) List() []domain.ToolInfo {
	return r.tools
}
