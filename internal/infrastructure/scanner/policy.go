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

func NewToolPolicy() *ToolPolicy {
	return &ToolPolicy{
		allowedTools: map[string]bool{
			"nmap_scan": true,
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

	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("nmap_scan requires target")
	}
	if strings.ContainsAny(target, " \t\n\r") {
		return fmt.Errorf("target must not contain spaces")
	}

	if ports, ok := intent.Params["ports"].(string); ok {
		ports = strings.TrimSpace(ports)
		if ports != "" && !portsPattern.MatchString(ports) {
			return fmt.Errorf("ports must contain only digits, comma, and dash")
		}
	}

	if topPortsRaw, ok := intent.Params["top_ports"]; ok {
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

	if v, ok := intent.Params["service_detection"]; ok {
		if _, castOK := v.(bool); !castOK {
			return fmt.Errorf("service_detection must be boolean")
		}
	}

	return nil
}
