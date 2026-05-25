package scanner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"apant_be/internal/domain"
)

type MultiExecutorConfig struct {
	NmapBinary      string
	NmapTimeout     time.Duration
	Timeout         time.Duration
	NucleiTemplates string
	WordlistsDir    string
}

type MultiExecutor struct {
	nmapBinary      string
	nmapTimeout     time.Duration
	timeout         time.Duration
	nucleiTemplates string
	wordlistsDir    string
}

func NewMultiExecutor(cfg MultiExecutorConfig) *MultiExecutor {
	nmapBinary := strings.TrimSpace(cfg.NmapBinary)
	if nmapBinary == "" {
		nmapBinary = "nmap"
	}

	nmapTimeout := cfg.NmapTimeout
	if nmapTimeout <= 0 {
		nmapTimeout = 60 * time.Second
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 300 * time.Second
	}

	nucleiTemplates := strings.TrimSpace(cfg.NucleiTemplates)
	if nucleiTemplates == "" {
		nucleiTemplates = "/home/scanner/.nuclei-templates"
	}

	wordlistsDir := strings.TrimSpace(cfg.WordlistsDir)
	if wordlistsDir == "" {
		wordlistsDir = "/wordlists"
	}

	return &MultiExecutor{
		nmapBinary:      nmapBinary,
		nmapTimeout:     nmapTimeout,
		timeout:         timeout,
		nucleiTemplates: nucleiTemplates,
		wordlistsDir:    wordlistsDir,
	}
}

func (e *MultiExecutor) Execute(intent *domain.ToolIntent) map[string]any {
	if intent == nil {
		return map[string]any{"status": "error", "error": "nil tool intent"}
	}

	name := strings.TrimSpace(strings.ToLower(intent.Name))

	switch name {
	case "nmap_scan":
		return e.executeNmap(intent)
	case "httpx_probe":
		return e.executeGeneric("httpx", intent, e.buildHttpxArgs)
	case "subfinder_enum":
		return e.executeGeneric("subfinder", intent, e.buildSubfinderArgs)
	case "katana_crawl":
		return e.executeGeneric("katana", intent, e.buildKatanaArgs)
	case "gau_urls":
		return e.executeGeneric("gau", intent, e.buildGauArgs)
	case "waybackurls_fetch":
		return e.executeGeneric("waybackurls", intent, e.buildWaybackurlsArgs)
	case "ffuf_fuzz":
		return e.executeGeneric("ffuf", intent, e.buildFfufArgs)
	case "nuclei_scan":
		return e.executeGeneric("nuclei", intent, e.buildNucleiArgs)
	case "dalfox_xss":
		return e.executeGeneric("dalfox", intent, e.buildDalfoxArgs)
	case "sqlmap_scan":
		return e.executeGeneric("sqlmap", intent, e.buildSqlmapArgs)
	case "http_request":
		return e.executeHTTPRequest(intent)
	case "mitmdump_intercept":
		return e.executeGeneric("mitmdump", intent, e.buildMitmdumpArgs)
	default:
		return map[string]any{"status": "error", "tool": name, "error": "tool not implemented"}
	}
}

func (e *MultiExecutor) executeNmap(intent *domain.ToolIntent) map[string]any {
	local := NewLocalNmapExecutor(LocalNmapConfig{
		Binary:  e.nmapBinary,
		Timeout: e.nmapTimeout,
	})
	return local.Execute(intent)
}

func (e *MultiExecutor) executeGeneric(
	toolName string,
	intent *domain.ToolIntent,
	buildArgs func(*domain.ToolIntent) ([]string, error),
) map[string]any {
	args, err := buildArgs(intent)
	if err != nil {
		return map[string]any{"status": "error", "tool": toolName, "error": err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, toolName, args...)

	if toolName == "gau" || toolName == "waybackurls" {
		target, _ := intent.Params["target"].(string)
		cmd.Stdin = strings.NewReader(strings.TrimSpace(target) + "\n")
	}

	output, execErr := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return map[string]any{
			"status":          "error",
			"tool":            toolName,
			"error":           fmt.Sprintf("%s timed out", toolName),
			"timeout_seconds": int(e.timeout.Seconds()),
		}
	}

	outputStr := strings.TrimSpace(string(output))

	if execErr != nil {
		if outputStr == "" {
			return map[string]any{
				"status": "error",
				"tool":   toolName,
				"error":  fmt.Sprintf("failed to run %s: %v", toolName, execErr),
				"stderr": outputStr,
			}
		}
	}

	lines := parseLines(outputStr)

	return map[string]any{
		"status": "success",
		"tool":   toolName,
		"target": intent.Params["target"],
		"count":  len(lines),
		"output": lines,
		"raw":    outputStr,
	}
}

func (e *MultiExecutor) executeHTTPRequest(intent *domain.ToolIntent) map[string]any {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return map[string]any{"status": "error", "tool": "http_request", "error": "target is required"}
	}

	method := "GET"
	if m, ok := intent.Params["method"].(string); ok && m != "" {
		method = strings.ToUpper(strings.TrimSpace(m))
	}

	var bodyReader io.Reader
	if body, ok := intent.Params["body"].(string); ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return map[string]any{"status": "error", "tool": "http_request", "error": err.Error()}
	}

	if headers, ok := intent.Params["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	if cookie, ok := intent.Params["cookie"].(string); ok && cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; APANT-Scanner/1.0)")
	}

	client := &http.Client{
		Timeout: e.timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return map[string]any{"status": "error", "tool": "http_request", "error": err.Error()}
	}
	defer resp.Body.Close()

	limitedReader := io.LimitReader(resp.Body, 50*1024)
	respBody, _ := io.ReadAll(limitedReader)
	respBodyStr := string(respBody)

	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	successIndicators := []string{
		"Congratulations",
		"congratulations",
		"solved",
		"Solved",
		"level solved",
		"lab solved",
		"You solved",
		"you solved",
	}
	exploitSuccess := false
	lowerBody := strings.ToLower(respBodyStr)
	if !strings.Contains(lowerBody, "not solved") {
		for _, indicator := range successIndicators {
			if strings.Contains(respBodyStr, indicator) {
				exploitSuccess = true
				break
			}
		}
	}

	if location := resp.Header.Get("Location"); location != "" {
		respHeaders["Location"] = location
		if strings.Contains(strings.ToLower(location), "/my-account") {
			exploitSuccess = true
		}
	}

	previewLen := 50000
	bodyPreview := respBodyStr
	if len(bodyPreview) > previewLen {
		bodyPreview = bodyPreview[:previewLen] + "...[truncated]"
	}

	return map[string]any{
		"status":           "success",
		"tool":             "http_request",
		"target":           target,
		"method":           method,
		"status_code":      resp.StatusCode,
		"response_headers": respHeaders,
		"body_preview":     bodyPreview,
		"body_length":      len(respBodyStr),
		"exploit_success":  exploitSuccess,
	}
}

func parseLines(output string) []string {
	lines := strings.Split(output, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func (e *MultiExecutor) buildHttpxArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("httpx_probe requires target")
	}

	args := []string{"-u", target, "-silent", "-title", "-status-code", "-tech-detect", "-json"}

	if followRedirects, ok := intent.Params["follow_redirects"].(bool); ok && followRedirects {
		args = append(args, "-follow-redirects")
	}

	return args, nil
}

func (e *MultiExecutor) buildSubfinderArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("subfinder_enum requires target")
	}

	args := []string{"-d", target, "-silent"}

	if all, ok := intent.Params["all"].(bool); ok && all {
		args = append(args, "-all")
	}

	return args, nil
}

func (e *MultiExecutor) buildKatanaArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("katana_crawl requires target")
	}

	depth := 2
	if d, ok := intent.Params["depth"]; ok {
		switch v := d.(type) {
		case float64:
			depth = int(v)
		case int:
			depth = v
		}
	}

	args := []string{"-u", target, "-depth", fmt.Sprintf("%d", depth), "-silent"}

	return args, nil
}

func (e *MultiExecutor) buildGauArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("gau_urls requires target")
	}

	args := []string{"--blacklist", "jpg,jpeg,png,gif,svg,ico,css,woff,woff2,ttf"}

	if subs, ok := intent.Params["include_subdomains"].(bool); ok && subs {
		args = append(args, "--subs")
	}

	return args, nil
}

func (e *MultiExecutor) buildWaybackurlsArgs(_ *domain.ToolIntent) ([]string, error) {
	return []string{}, nil
}

func (e *MultiExecutor) buildFfufArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("ffuf_fuzz requires target")
	}

	if !strings.Contains(target, "FUZZ") {
		target = strings.TrimRight(target, "/") + "/FUZZ"
	}

	wordlist, _ := intent.Params["wordlist"].(string)
	wordlist = strings.TrimSpace(wordlist)
	if wordlist == "" {
		wordlist = e.wordlistsDir + "/common.txt"
	}

	if !strings.HasPrefix(wordlist, "/wordlists/") {
		wordlist = e.wordlistsDir + "/common.txt"
	}

	threads := 10
	if t, ok := intent.Params["threads"]; ok {
		switch v := t.(type) {
		case float64:
			threads = int(v)
		case int:
			threads = v
		}
	}

	args := []string{
		"-u", target,
		"-w", wordlist,
		"-mc", "200,201,301,302,403",
		"-t", fmt.Sprintf("%d", threads),
		"-s",
		"-json",
	}

	return args, nil
}

func (e *MultiExecutor) buildNucleiArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("nuclei_scan requires target")
	}

	templatesPath := e.nucleiTemplates + "/http/"

	if tp, ok := intent.Params["templates"].(string); ok && tp != "" {
		tp = strings.TrimSpace(tp)
		if strings.HasPrefix(tp, e.nucleiTemplates) {
			templatesPath = tp
		}
	}

	args := []string{"-u", target, "-t", templatesPath, "-silent"}

	if severity, ok := intent.Params["severity"].(string); ok && severity != "" {
		severity = strings.ReplaceAll(strings.TrimSpace(severity), "/", ",")
		args = append(args, "-severity", severity)
	}

	return args, nil
}

func (e *MultiExecutor) buildDalfoxArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("dalfox_xss requires target")
	}

	args := []string{"url", target, "--silence"}

	if cookie, ok := intent.Params["cookie"].(string); ok && cookie != "" {
		args = append(args, "--cookie", strings.TrimSpace(cookie))
	}

	return args, nil
}

func (e *MultiExecutor) buildSqlmapArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("sqlmap_scan requires target")
	}

	level := 1
	if l, ok := intent.Params["level"]; ok {
		switch v := l.(type) {
		case float64:
			level = int(v)
		case int:
			level = v
		}
	}

	risk := 1
	if r, ok := intent.Params["risk"]; ok {
		switch v := r.(type) {
		case float64:
			risk = int(v)
		case int:
			risk = v
		}
	}

	args := []string{
		"-u", target,
		"--batch",
		fmt.Sprintf("--level=%d", level),
		fmt.Sprintf("--risk=%d", risk),
		"--output-dir=/home/scanner/.sqlmap/output",
		"--flush-session",
		"--fresh-queries",
		"--technique=BEUST",
	}

	return args, nil
}

func (e *MultiExecutor) buildMitmdumpArgs(intent *domain.ToolIntent) ([]string, error) {
	port := "8080"
	if p, ok := intent.Params["port"].(string); ok && p != "" {
		port = strings.TrimSpace(p)
	} else if p, ok := intent.Params["port"].(float64); ok {
		port = fmt.Sprintf("%d", int(p))
	}

	duration := 30
	if d, ok := intent.Params["duration_seconds"]; ok {
		switch v := d.(type) {
		case float64:
			duration = int(v)
		case int:
			duration = v
		}
	}

	_ = duration

	args := []string{
		"--listen-port", port,
		"--flow-detail", "1",
		"--quiet",
	}

	return args, nil
}
