package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"apant_be/internal/domain"
)

// SAST (static analysis) tools. Unlike the dynamic tools these never touch the
// network target; they operate on an uploaded source tree that the API has
// extracted into <workspaceRoot>/<scan_id>/src. Every file path is confined to
// that directory — the agent cannot read anything outside its own workspace.

const (
	maxReadFileBytes    = 256 * 1024
	maxListFiles        = 3000
	maxGrepMatches      = 300
	maxSemgrepFindings  = 400
	maxGitleaksFindings = 200
	maxOSVFindings      = 300
	maxSnippetLen       = 600
)

// workspaceIDPattern restricts scan_id to a single safe path segment so it can
// never be used to traverse out of the workspace root.
var workspaceIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// resolveWorkspace returns the validated absolute source directory for a scan.
func (e *MultiExecutor) resolveWorkspace(intent *domain.ToolIntent) (string, error) {
	scanID, _ := intent.Params["scan_id"].(string)
	scanID = strings.TrimSpace(scanID)
	if scanID == "" {
		return "", fmt.Errorf("scan_id is required")
	}
	if !workspaceIDPattern.MatchString(scanID) {
		return "", fmt.Errorf("invalid scan_id")
	}

	base := filepath.Join(e.workspaceRoot, scanID, "src")
	if !isWithin(e.workspaceRoot, base) {
		return "", fmt.Errorf("workspace path escapes root")
	}

	info, err := os.Stat(base)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("workspace not found for scan")
	}
	return base, nil
}

// resolveInside joins a caller-supplied relative path onto base and guarantees
// the result stays within base (defense against path traversal).
func resolveInside(base, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return base, nil
	}
	rel = filepath.FromSlash(rel)
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative to the repository root")
	}
	clean := filepath.Clean(filepath.Join(base, rel))
	if !isWithin(base, clean) {
		return "", fmt.Errorf("path escapes the workspace")
	}
	return clean, nil
}

// isWithin reports whether p is root itself or a descendant of root.
func isWithin(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..")
}

func staticErr(tool, msg string) map[string]any {
	return map[string]any{"status": "error", "tool": tool, "error": msg}
}

// runStaticCmd executes a tool capturing stdout and stderr separately. SAST
// tools commonly exit non-zero when they find issues, so the caller decides what
// a non-zero code means rather than treating it as a hard failure.
func (e *MultiExecutor) runStaticCmd(name string, args ...string) (stdout string, stderr string, err error, timedOut bool) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return "", "", runErr, true
	}
	return outBuf.String(), errBuf.String(), runErr, false
}

// ---------------------------------------------------------------------------
// File navigation tools
// ---------------------------------------------------------------------------

func (e *MultiExecutor) executeListFiles(intent *domain.ToolIntent) map[string]any {
	base, err := e.resolveWorkspace(intent)
	if err != nil {
		return staticErr("list_files", err.Error())
	}

	sub, _ := intent.Params["path"].(string)
	root, err := resolveInside(base, sub)
	if err != nil {
		return staticErr("list_files", err.Error())
	}

	files := make([]string, 0, 256)
	truncated := false
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		if len(files) >= maxListFiles {
			truncated = true
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return staticErr("list_files", walkErr.Error())
	}

	sort.Strings(files)
	return map[string]any{
		"status":           "success",
		"tool":             "list_files",
		"count":            len(files),
		"output":           files,
		"output_truncated": truncated,
	}
}

func (e *MultiExecutor) executeReadFile(intent *domain.ToolIntent) map[string]any {
	base, err := e.resolveWorkspace(intent)
	if err != nil {
		return staticErr("read_file", err.Error())
	}

	rel, _ := intent.Params["path"].(string)
	if strings.TrimSpace(rel) == "" {
		return staticErr("read_file", "path is required")
	}
	full, err := resolveInside(base, rel)
	if err != nil {
		return staticErr("read_file", err.Error())
	}

	info, err := os.Lstat(full)
	if err != nil {
		return staticErr("read_file", "file not found")
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return staticErr("read_file", "not a regular file")
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return staticErr("read_file", err.Error())
	}

	fileTruncated := false
	if len(data) > maxReadFileBytes {
		data = data[:maxReadFileBytes]
		fileTruncated = true
	}

	content := string(data)
	start := paramInt(intent.Params, "start_line", 0)
	end := paramInt(intent.Params, "end_line", 0)
	if start > 0 || end > 0 {
		content, fileTruncated = sliceLines(content, start, end, fileTruncated)
	}

	return map[string]any{
		"status":     "success",
		"tool":       "read_file",
		"path":       filepath.ToSlash(rel),
		"size_bytes": info.Size(),
		"truncated":  fileTruncated,
		"content":    content,
	}
}

func (e *MultiExecutor) executeGrepCode(intent *domain.ToolIntent) map[string]any {
	base, err := e.resolveWorkspace(intent)
	if err != nil {
		return staticErr("grep_code", err.Error())
	}

	pattern, _ := intent.Params["pattern"].(string)
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return staticErr("grep_code", "pattern is required")
	}

	searchDir := base
	if sub, _ := intent.Params["path"].(string); strings.TrimSpace(sub) != "" {
		searchDir, err = resolveInside(base, sub)
		if err != nil {
			return staticErr("grep_code", err.Error())
		}
	}

	// "--" terminates flag parsing so a pattern starting with "-" is treated as a
	// literal pattern, never an option.
	args := []string{
		"--line-number", "--no-heading", "--color", "never",
		"--max-count", "20", "--max-columns", "300", "--smart-case",
	}
	if glob, ok := intent.Params["glob"].(string); ok && strings.TrimSpace(glob) != "" {
		args = append(args, "--glob", strings.TrimSpace(glob))
	}
	args = append(args, "--", pattern, searchDir)

	stdout, stderr, runErr, timedOut := e.runStaticCmd("rg", args...)
	if timedOut {
		return map[string]any{"status": "error", "tool": "grep_code", "error": "grep_code timed out"}
	}
	// ripgrep exits 1 when there are no matches — that is not an error.
	if runErr != nil && strings.TrimSpace(stdout) == "" && strings.TrimSpace(stderr) != "" {
		return staticErr("grep_code", strings.TrimSpace(firstLine(stderr)))
	}

	matches := make([]string, 0, 64)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches = append(matches, strings.TrimPrefix(line, base+string(os.PathSeparator)))
		if len(matches) >= maxGrepMatches {
			break
		}
	}

	return map[string]any{
		"status": "success",
		"tool":   "grep_code",
		"count":  len(matches),
		"output": matches,
	}
}

// ---------------------------------------------------------------------------
// Deterministic analysis tools
// ---------------------------------------------------------------------------

func (e *MultiExecutor) executeSemgrep(intent *domain.ToolIntent) map[string]any {
	base, err := e.resolveWorkspace(intent)
	if err != nil {
		return staticErr("semgrep", err.Error())
	}

	config := e.semgrepConfig
	args := []string{
		"--json", "--quiet", "--metrics=off", "--disable-version-check",
		"--timeout", "30", "--max-target-bytes", "2000000",
		"--config", config, base,
	}

	stdout, stderr, _, timedOut := e.runStaticCmd("semgrep", args...)
	if timedOut {
		return map[string]any{"status": "error", "tool": "semgrep", "error": "semgrep timed out"}
	}
	if strings.TrimSpace(stdout) == "" {
		return staticErr("semgrep", "semgrep produced no output: "+firstLine(stderr))
	}

	var parsed struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			End struct {
				Line int `json:"line"`
			} `json:"end"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
				Lines    string `json:"lines"`
				Metadata struct {
					Cwe        any    `json:"cwe"`
					OwaspField any    `json:"owasp"`
					Confidence string `json:"confidence"`
				} `json:"metadata"`
			} `json:"extra"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		return staticErr("semgrep", "failed to parse semgrep output")
	}

	findings := make([]map[string]any, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		findings = append(findings, map[string]any{
			"rule_id":    r.CheckID,
			"path":       relWorkspacePath(base, r.Path),
			"start_line": r.Start.Line,
			"end_line":   r.End.Line,
			"severity":   strings.ToLower(strings.TrimSpace(r.Extra.Severity)),
			"message":    strings.TrimSpace(r.Extra.Message),
			"snippet":    clampSnippet(r.Extra.Lines),
			"confidence": strings.ToLower(strings.TrimSpace(r.Extra.Metadata.Confidence)),
		})
		if len(findings) >= maxSemgrepFindings {
			break
		}
	}

	return map[string]any{
		"status":   "success",
		"tool":     "semgrep",
		"count":    len(findings),
		"findings": findings,
	}
}

func (e *MultiExecutor) executeGitleaks(intent *domain.ToolIntent) map[string]any {
	base, err := e.resolveWorkspace(intent)
	if err != nil {
		return staticErr("gitleaks", err.Error())
	}

	report, err := os.CreateTemp("", "gitleaks-*.json")
	if err != nil {
		return staticErr("gitleaks", "cannot create report file")
	}
	reportPath := report.Name()
	_ = report.Close()
	defer os.Remove(reportPath)

	args := []string{
		"detect", "--source", base, "--no-git", "--no-banner",
		"--report-format", "json", "--report-path", reportPath, "--exit-code", "0",
	}
	_, stderr, _, timedOut := e.runStaticCmd("gitleaks", args...)
	if timedOut {
		return map[string]any{"status": "error", "tool": "gitleaks", "error": "gitleaks timed out"}
	}

	data, readErr := os.ReadFile(reportPath)
	if readErr != nil {
		return staticErr("gitleaks", "cannot read report: "+firstLine(stderr))
	}

	var leaks []struct {
		Description string `json:"Description"`
		File        string `json:"File"`
		StartLine   int    `json:"StartLine"`
		EndLine     int    `json:"EndLine"`
		RuleID      string `json:"RuleID"`
		Match       string `json:"Match"`
	}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &leaks); err != nil {
			return staticErr("gitleaks", "failed to parse gitleaks output")
		}
	}

	findings := make([]map[string]any, 0, len(leaks))
	for _, l := range leaks {
		findings = append(findings, map[string]any{
			"rule_id":     l.RuleID,
			"description": strings.TrimSpace(l.Description),
			"path":        relWorkspacePath(base, l.File),
			"start_line":  l.StartLine,
			"end_line":    l.EndLine,
			"match":       redactSecret(l.Match),
		})
		if len(findings) >= maxGitleaksFindings {
			break
		}
	}

	return map[string]any{
		"status":   "success",
		"tool":     "gitleaks",
		"count":    len(findings),
		"findings": findings,
	}
}

func (e *MultiExecutor) executeOSVScan(intent *domain.ToolIntent) map[string]any {
	base, err := e.resolveWorkspace(intent)
	if err != nil {
		return staticErr("osv_scan", err.Error())
	}

	stdout, stderr, _, timedOut := e.runStaticCmd("osv-scanner", "--format", "json", "--recursive", base)
	if timedOut {
		return map[string]any{"status": "error", "tool": "osv_scan", "error": "osv_scan timed out"}
	}
	if strings.TrimSpace(stdout) == "" {
		// No manifests or no connectivity — report empty rather than failing the run.
		return map[string]any{
			"status": "success", "tool": "osv_scan", "count": 0,
			"findings": []map[string]any{}, "note": firstLine(stderr),
		}
	}

	var parsed struct {
		Results []struct {
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
			Packages []struct {
				Package struct {
					Name      string `json:"name"`
					Version   string `json:"version"`
					Ecosystem string `json:"ecosystem"`
				} `json:"package"`
				Vulnerabilities []struct {
					ID      string `json:"id"`
					Summary string `json:"summary"`
				} `json:"vulnerabilities"`
				Groups []struct {
					MaxSeverity string `json:"max_severity"`
				} `json:"groups"`
			} `json:"packages"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		return staticErr("osv_scan", "failed to parse osv-scanner output")
	}

	findings := make([]map[string]any, 0)
	for _, res := range parsed.Results {
		manifest := relWorkspacePath(base, res.Source.Path)
		for _, pkg := range res.Packages {
			severity := ""
			if len(pkg.Groups) > 0 {
				severity = pkg.Groups[0].MaxSeverity
			}
			for _, v := range pkg.Vulnerabilities {
				findings = append(findings, map[string]any{
					"id":        v.ID,
					"package":   pkg.Package.Name,
					"version":   pkg.Package.Version,
					"ecosystem": pkg.Package.Ecosystem,
					"summary":   clampSnippet(v.Summary),
					"severity":  severity,
					"path":      manifest,
				})
				if len(findings) >= maxOSVFindings {
					break
				}
			}
		}
	}

	return map[string]any{
		"status":   "success",
		"tool":     "osv_scan",
		"count":    len(findings),
		"findings": findings,
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".venv", "venv", "__pycache__",
		"dist", "build", ".next", ".idea", ".vscode", "target":
		return true
	}
	return false
}

func relWorkspacePath(base, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if rel, err := filepath.Rel(base, p); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(strings.TrimPrefix(p, base+string(os.PathSeparator)))
}

func clampSnippet(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxSnippetLen {
		return s[:maxSnippetLen] + "...[truncated]"
	}
	return s
}

// redactSecret keeps just enough of a matched secret to be recognizable without
// leaking the full credential into the report.
func redactSecret(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-2:]
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func paramInt(params map[string]any, key string, fallback int) int {
	v, ok := params[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return fallback
}

func sliceLines(content string, start, end int, alreadyTruncated bool) (string, bool) {
	lines := strings.Split(content, "\n")
	if start < 1 {
		start = 1
	}
	if end < start || end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		return "", alreadyTruncated
	}
	return strings.Join(lines[start-1:end], "\n"), alreadyTruncated || end < len(lines)
}
