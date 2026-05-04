package scanner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"apant_be/internal/domain"
)

type LocalNmapConfig struct {
	Binary  string
	Timeout time.Duration
}

type LocalNmapExecutor struct {
	binary  string
	timeout time.Duration
}

func NewLocalNmapExecutor(cfg LocalNmapConfig) *LocalNmapExecutor {
	binary := strings.TrimSpace(cfg.Binary)
	if binary == "" {
		binary = "nmap"
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &LocalNmapExecutor{binary: binary, timeout: timeout}
}

func (e *LocalNmapExecutor) Execute(intent *domain.ToolIntent) map[string]any {
	if intent == nil {
		return map[string]any{"status": "error", "error": "nil tool intent"}
	}

	name := strings.TrimSpace(strings.ToLower(intent.Name))
	if name != "nmap_scan" {
		return map[string]any{
			"status": "error",
			"tool":   name,
			"error":  "tool is not implemented by local executor",
		}
	}

	target, nmapArgs, serviceDetection, err := buildNmapArgs(intent)
	if err != nil {
		return map[string]any{"status": "error", "tool": "nmap_scan", "error": err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.binary, nmapArgs...)
	output, execErr := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return map[string]any{
			"status":          "error",
			"tool":            "nmap_scan",
			"error":           "nmap scan timed out",
			"timeout_seconds": int(e.timeout.Seconds()),
		}
	}

	if execErr != nil {
		return map[string]any{
			"status": "error",
			"tool":   "nmap_scan",
			"error":  fmt.Sprintf("failed to run nmap: %v", execErr),
			"stderr": strings.TrimSpace(string(output)),
		}
	}

	parsed, err := parseNmapXML(output)
	if err != nil {
		return map[string]any{
			"status": "error",
			"tool":   "nmap_scan",
			"error":  fmt.Sprintf("failed to parse nmap xml: %v", err),
			"output": strings.TrimSpace(string(output)),
		}
	}

	return map[string]any{
		"status":           "success",
		"tool":             "nmap_scan",
		"engine":           "local",
		"binary":           e.binary,
		"target":           target,
		"service_detected": serviceDetection,
		"nmap_args":        nmapArgs,
		"open_port_count":  parsed.OpenPortCount,
		"hosts":            parsed.Hosts,
	}
}
