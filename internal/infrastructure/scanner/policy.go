package scanner

import (
	"fmt"
	"regexp"
	"strings"

	"apant_be/internal/domain"
)

type ToolPolicy struct {
	allowedTools map[string]bool
}

var portsPattern = regexp.MustCompile(`^[0-9,-]+$`)
var scanIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// staticTools operate on an uploaded source tree instead of a network target, so
// they are exempt from the target/host validation the dynamic tools require.
var staticTools = map[string]bool{
	"semgrep_scan":  true,
	"gitleaks_scan": true,
	"osv_scan":      true,
	"list_files":    true,
	"read_file":     true,
	"grep_code":     true,
}

func NewToolPolicy() *ToolPolicy {
	return &ToolPolicy{
		allowedTools: map[string]bool{
			"nmap_scan":          true,
			"wafw00f_detect":     true,
			"httpx_probe":        true,
			"subfinder_enum":     true,
			"katana_crawl":       true,
			"gau_urls":           true,
			"waybackurls_fetch":  true,
			"ffuf_fuzz":          true,
			"nuclei_scan":        true,
			"dalfox_xss":         true,
			"sqlmap_scan":        true,
			"http_request":       true,
			"mitmdump_intercept": true,
			"semgrep_scan":       true,
			"gitleaks_scan":      true,
			"osv_scan":           true,
			"list_files":         true,
			"read_file":          true,
			"grep_code":          true,
		},
	}
}

func (p *ToolPolicy) Validate(intent *domain.ToolIntent) error {
	if intent == nil {
		return fmt.Errorf("tool intent is nil")
	}

	name := strings.TrimSpace(strings.ToLower(intent.Name))
	if name == "" {
		return fmt.Errorf("tool name is required")
	}
	if !p.allowedTools[name] {
		return fmt.Errorf("tool is not allowed: %s", name)
	}

	if staticTools[name] {
		return validateStaticParams(name, intent.Params)
	}

	if name != "mitmdump_intercept" {
		target, _ := intent.Params["target"].(string)
		target = strings.TrimSpace(target)
		if target == "" {
			return fmt.Errorf("%s requires target", name)
		}
		if strings.ContainsAny(target, " \t\n\r") {
			return fmt.Errorf("target must not contain spaces")
		}
	}

	switch name {
	case "nmap_scan":
		return validateNmapParams(intent.Params)
	case "ffuf_fuzz":
		return validateFfufParams(intent.Params)
	case "nuclei_scan":
		return validateNucleiParams(intent.Params)
	case "sqlmap_scan":
		return validateSqlmapParams(intent.Params)
	case "http_request":
		return validateHTTPRequestParams(intent.Params)
	}

	return nil
}

func validateNmapParams(params map[string]any) error {
	if ports, ok := params["ports"].(string); ok {
		ports = strings.TrimSpace(ports)
		if ports != "" && !portsPattern.MatchString(ports) {
			return fmt.Errorf("ports must contain only digits, comma, and dash")
		}
	}

	if topPortsRaw, ok := params["top_ports"]; ok {
		var topPorts int
		switch v := topPortsRaw.(type) {
		case float64:
			topPorts = int(v)
		case int:
			topPorts = v
		default:
			return fmt.Errorf("top_ports must be a number")
		}
		if topPorts < 1 || topPorts > 1000 {
			return fmt.Errorf("top_ports must be between 1 and 1000")
		}
	}

	if v, ok := params["service_detection"]; ok {
		if _, castOK := v.(bool); !castOK {
			return fmt.Errorf("service_detection must be boolean")
		}
	}

	return nil
}

func validateFfufParams(params map[string]any) error {
	if threads, ok := params["threads"]; ok {
		var t int
		switch v := threads.(type) {
		case float64:
			t = int(v)
		case int:
			t = v
		default:
			return fmt.Errorf("threads must be a number")
		}
		if t < 1 || t > 200 {
			return fmt.Errorf("threads must be between 1 and 200")
		}
	}

	return nil
}

func validateNucleiParams(params map[string]any) error {
	if severity, ok := params["severity"].(string); ok {
		severity = strings.TrimSpace(strings.ToLower(severity))
		allowed := map[string]bool{
			"critical": true,
			"high":     true,
			"medium":   true,
			"low":      true,
			"info":     true,
		}
		parts := []string{severity}
		if strings.Contains(severity, "/") {
			parts = strings.Split(severity, "/")
		} else if strings.Contains(severity, ",") {
			parts = strings.Split(severity, ",")
		}
		for _, s := range parts {
			s = strings.TrimSpace(s)
			if s != "" && !allowed[s] {
				return fmt.Errorf("invalid severity: %s", s)
			}
		}
	}

	// Only a curated set of nuclei tags is permitted. The agent is an LLM, so an
	// unconstrained tag (e.g. "cve") would execute thousands of templates; restrict
	// it to the CMS tag(s) we have validated. keep in sync with buildNucleiArgs.
	if tags, ok := params["tags"].(string); ok && strings.TrimSpace(tags) != "" {
		allowedTags := map[string]bool{"wordpress": true}
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(strings.ToLower(t))
			if t != "" && !allowedTags[t] {
				return fmt.Errorf("tag not allowed: %s", t)
			}
		}
	}

	return nil
}

func validateSqlmapParams(params map[string]any) error {
	if level, ok := params["level"]; ok {
		var l int
		switch v := level.(type) {
		case float64:
			l = int(v)
		case int:
			l = v
		default:
			return fmt.Errorf("level must be a number")
		}
		if l < 1 || l > 5 {
			return fmt.Errorf("level must be between 1 and 5")
		}
	}

	if risk, ok := params["risk"]; ok {
		var r int
		switch v := risk.(type) {
		case float64:
			r = int(v)
		case int:
			r = v
		default:
			return fmt.Errorf("risk must be a number")
		}
		if r < 1 || r > 3 {
			return fmt.Errorf("risk must be between 1 and 3")
		}
	}

	return nil
}

// validateStaticParams enforces that SAST tools carry a well-formed scan_id (the
// API injects this; it must never be attacker-controlled to a different value)
// and that each tool's required parameters are present. Path containment itself
// is enforced again in the scanner executor — this is the first of two gates.
func validateStaticParams(name string, params map[string]any) error {
	scanID, _ := params["scan_id"].(string)
	scanID = strings.TrimSpace(scanID)
	if scanID == "" {
		return fmt.Errorf("%s requires scan_id", name)
	}
	if !scanIDPattern.MatchString(scanID) {
		return fmt.Errorf("invalid scan_id")
	}

	switch name {
	case "read_file":
		if path, _ := params["path"].(string); strings.TrimSpace(path) == "" {
			return fmt.Errorf("read_file requires path")
		}
	case "grep_code":
		if pattern, _ := params["pattern"].(string); strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("grep_code requires pattern")
		}
	}

	return nil
}

func validateHTTPRequestParams(params map[string]any) error {
	target, _ := params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("http_request requires target")
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		return fmt.Errorf("http_request target must start with http:// or https://")
	}

	if method, ok := params["method"].(string); ok {
		method = strings.ToUpper(strings.TrimSpace(method))
		allowed := map[string]bool{
			"GET":    true,
			"POST":   true,
			"PUT":    true,
			"DELETE": true,
			"PATCH":  true,
			"HEAD":   true,
		}
		if method != "" && !allowed[method] {
			return fmt.Errorf("http_request method not allowed: %s", method)
		}
	}

	return nil
}
