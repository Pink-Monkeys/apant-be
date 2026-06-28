package scanner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
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
	WorkspaceRoot   string
	SemgrepConfig   string
}

type MultiExecutor struct {
	nmapBinary      string
	nmapTimeout     time.Duration
	timeout         time.Duration
	nucleiTemplates string
	wordlistsDir    string
	workspaceRoot   string
	semgrepConfig   string
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

	workspaceRoot := strings.TrimSpace(cfg.WorkspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = "/workspace"
	}

	semgrepConfig := strings.TrimSpace(cfg.SemgrepConfig)
	if semgrepConfig == "" {
		semgrepConfig = "/opt/semgrep-rules"
	}

	return &MultiExecutor{
		nmapBinary:      nmapBinary,
		nmapTimeout:     nmapTimeout,
		timeout:         timeout,
		nucleiTemplates: nucleiTemplates,
		wordlistsDir:    wordlistsDir,
		workspaceRoot:   workspaceRoot,
		semgrepConfig:   semgrepConfig,
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
	case "semgrep_scan":
		return e.executeSemgrep(intent)
	case "gitleaks_scan":
		return e.executeGitleaks(intent)
	case "osv_scan":
		return e.executeOSVScan(intent)
	case "list_files":
		return e.executeListFiles(intent)
	case "read_file":
		return e.executeReadFile(intent)
	case "grep_code":
		return e.executeGrepCode(intent)
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

	bodyStr, _ := intent.Params["body"].(string)
	var bodyReader io.Reader
	if bodyStr != "" {
		bodyReader = strings.NewReader(bodyStr)
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
		if isLikelyAuthBypassRedirect(method, bodyStr, target, resp.StatusCode, location) {
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

// isLikelyAuthBypassRedirect flags a redirect that most likely indicates a
// successful authentication bypass. The legacy PortSwigger "/my-account" signal
// is kept as a known-lab marker. Beyond that, a generalized signal fires when a
// form submission (POST) carrying a SQL-injection payload — instead of bouncing
// back to the login/error page — redirects elsewhere, which is the classic shape
// of a login bypass on a non-lab target.
func isLikelyAuthBypassRedirect(method, body, target string, status int, location string) bool {
	loc := strings.ToLower(strings.TrimSpace(location))
	if loc == "" {
		return false
	}
	if strings.Contains(loc, "/my-account") {
		return true
	}
	if status < 300 || status >= 400 {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(method), "POST") {
		return false
	}
	if !looksLikeSQLiPayload(body) && !looksLikeSQLiPayload(target) {
		return false
	}
	// A redirect back to an auth/error page means the bypass failed.
	for _, deny := range []string{"login", "signin", "sign-in", "logout", "auth", "error", "invalid", "denied", "failed"} {
		if strings.Contains(loc, deny) {
			return false
		}
	}
	return true
}

// looksLikeSQLiPayload reports whether s contains a recognizable SQL-injection
// authentication-bypass pattern, decoding URL/form encoding first so payloads
// carried in a POST body (e.g. %27+OR+%271%27%3D%271) are matched.
func looksLikeSQLiPayload(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	lower := strings.ToLower(s)
	if dec, err := url.QueryUnescape(lower); err == nil {
		lower = dec
	}
	lower = strings.ReplaceAll(lower, "+", " ")
	for _, p := range []string{
		"' or '1'='1", "' or 1=1", "or '1'='1", "or 1=1", "' or ''='",
		"\" or \"1\"=\"1", "') or ('1'='1", "'--", "'#", "'or'1'='1", "union select",
	} {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
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

// authFromParams extracts the session cookie and custom headers injected for an
// authenticated scan, so each tool can forward them in its own flag format.
func authFromParams(params map[string]any) (cookie string, headers []string) {
	cookie, _ = params["cookie"].(string)
	cookie = strings.TrimSpace(cookie)

	if h, ok := params["headers"].(map[string]any); ok {
		keys := make([]string, 0, len(h))
		for k := range h {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if v, ok := h[k].(string); ok && strings.TrimSpace(v) != "" {
				headers = append(headers, k+": "+strings.TrimSpace(v))
			}
		}
	}
	return cookie, headers
}

// appendAuthAsHeaders forwards auth via a single header flag (e.g. "-H"), for
// tools without a dedicated cookie option (httpx, katana, nuclei).
func appendAuthAsHeaders(args []string, headerFlag string, params map[string]any) []string {
	cookie, headers := authFromParams(params)
	for _, h := range headers {
		args = append(args, headerFlag, h)
	}
	if cookie != "" {
		args = append(args, headerFlag, "Cookie: "+cookie)
	}
	return args
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

	args = appendAuthAsHeaders(args, "-H", intent.Params)

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
	args = appendAuthAsHeaders(args, "-H", intent.Params)

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

	args = appendAuthAsHeaders(args, "-H", intent.Params)

	return args, nil
}

func (e *MultiExecutor) buildDalfoxArgs(intent *domain.ToolIntent) ([]string, error) {
	target, _ := intent.Params["target"].(string)
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("dalfox_xss requires target")
	}

	args := []string{"url", target, "--silence"}

	cookie, headers := authFromParams(intent.Params)
	if cookie != "" {
		args = append(args, "--cookie", cookie)
	}
	for _, h := range headers {
		args = append(args, "-H", h)
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

	cookie, headers := authFromParams(intent.Params)
	if cookie != "" {
		args = append(args, "--cookie="+cookie)
	}
	if len(headers) > 0 {
		args = append(args, "--headers="+strings.Join(headers, "\n"))
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
