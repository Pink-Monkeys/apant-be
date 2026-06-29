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
	return e.runStaticCmdEnv(nil, name, args...)
}

// runStaticCmdEnv is runStaticCmd with extra environment variables appended to
// the inherited environment (used to pass git safe.directory for history scans).
func (e *MultiExecutor) runStaticCmdEnv(extraEnv []string, name string, args ...string) (stdout string, stderr string, err error, timedOut bool) {
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
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

	// semgrepConfig names several rulesets (whitespace-separated): a curated APANT
	// pack plus the bundled OSS language rules. Each is run as its OWN semgrep
	// invocation rather than one shared --config: semgrep aborts the entire run if
	// any single config is invalid (e.g. a stray non-rule yaml in the OSS repo),
	// so isolating them guarantees one bad ruleset can never zero out the others.
	configs := strings.Fields(e.semgrepConfig)
	if len(configs) == 0 {
		configs = []string{"/opt/semgrep-rules"}
	}

	findings := make([]map[string]any, 0, 32)
	seen := make(map[string]bool)
	var lastErr string
	anyOK := false

	for _, cfg := range configs {
		cfgFindings, ferr := e.runSemgrepConfig(base, cfg)
		if ferr != "" {
			lastErr = ferr
			continue
		}
		anyOK = true
		for _, f := range cfgFindings {
			// Dedup identical hits reported by more than one ruleset.
			key := fmt.Sprintf("%v|%v|%v", f["path"], f["start_line"], f["rule_id"])
			if seen[key] {
				continue
			}
			seen[key] = true
			findings = append(findings, f)
			if len(findings) >= maxSemgrepFindings {
				break
			}
		}
		if len(findings) >= maxSemgrepFindings {
			break
		}
	}

	// Low/Informational misconfigurations (debug mode, verbose errors, info
	// disclosure) are detected directly and merged in, so they appear even if the
	// semgrep rulesets themselves found nothing.
	misconfig := detectMisconfigurations(base)
	for _, f := range misconfig {
		key := fmt.Sprintf("%v|%v|%v", f["path"], f["start_line"], f["rule_id"])
		if seen[key] {
			continue
		}
		seen[key] = true
		findings = append(findings, f)
		if len(findings) >= maxSemgrepFindings {
			break
		}
	}

	if !anyOK && len(misconfig) == 0 {
		return staticErr("semgrep", "all semgrep configs failed: "+lastErr)
	}

	return map[string]any{
		"status":   "success",
		"tool":     "semgrep",
		"count":    len(findings),
		"findings": findings,
	}
}

// runSemgrepConfig runs one semgrep ruleset over base and returns normalized
// findings. The second return is a non-empty error string if this config failed
// (so the caller can skip it and keep the others).
func (e *MultiExecutor) runSemgrepConfig(base, config string) ([]map[string]any, string) {
	args := []string{
		"--json", "--quiet", "--metrics=off", "--disable-version-check",
		"--timeout", "30", "--max-target-bytes", "2000000",
		"--config", config, base,
	}

	stdout, stderr, _, timedOut := e.runStaticCmd("semgrep", args...)
	if timedOut {
		return nil, "semgrep timed out"
	}
	if strings.TrimSpace(stdout) == "" {
		return nil, "semgrep produced no output: " + firstLine(stderr)
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
		return nil, "failed to parse semgrep output"
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
	}
	return findings, ""
}

func (e *MultiExecutor) executeGitleaks(intent *domain.ToolIntent) map[string]any {
	base, err := e.resolveWorkspace(intent)
	if err != nil {
		return staticErr("gitleaks", err.Error())
	}

	// Always scan the working tree (catches secrets in committed files like .env).
	findings, err := e.runGitleaks(base, false)
	if err != nil {
		return staticErr("gitleaks", err.Error())
	}

	// If the upload carried a .git directory, also mine the commit history: secrets
	// are frequently removed from the current tree but linger in old commits or
	// backup files (e.g. config.php.bak), which is itself a critical exposure.
	if dirExists(filepath.Join(base, ".git")) {
		if histFindings, herr := e.runGitleaks(base, true); herr == nil {
			findings = mergeGitleaksFindings(findings, histFindings)
		}
	}

	// Surface exposed sensitive files/dirs as findings in their own right — these
	// (.git, .env, *.bak) are high-value leaks a secret scanner alone may not flag
	// but are exactly what an attacker fetches first.
	findings = append(findings, detectExposedFiles(base)...)

	if len(findings) > maxGitleaksFindings {
		findings = findings[:maxGitleaksFindings]
	}

	return map[string]any{
		"status":   "success",
		"tool":     "gitleaks",
		"count":    len(findings),
		"findings": findings,
	}
}

// runGitleaks runs one gitleaks pass over base. withHistory=false scans the
// filesystem (current files); withHistory=true scans git commit history (requires
// a .git directory). Findings are normalized to the shared finding shape.
func (e *MultiExecutor) runGitleaks(base string, withHistory bool) ([]map[string]any, error) {
	report, err := os.CreateTemp("", "gitleaks-*.json")
	if err != nil {
		return nil, fmt.Errorf("cannot create report file")
	}
	reportPath := report.Name()
	_ = report.Close()
	defer os.Remove(reportPath)

	args := []string{
		"detect", "--source", base, "--no-banner",
		"--report-format", "json", "--report-path", reportPath, "--exit-code", "0",
	}
	// The workspace is written by the API process (different UID) and mounted
	// read-only, so git refuses history operations as "dubious ownership" unless
	// the directory is marked safe. These env vars inject that config for the
	// child git invocations gitleaks makes, without touching any global config.
	var extraEnv []string
	if !withHistory {
		args = append(args, "--no-git")
	} else {
		extraEnv = []string{
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=safe.directory",
			"GIT_CONFIG_VALUE_0=*",
		}
	}

	_, stderr, _, timedOut := e.runStaticCmdEnv(extraEnv, "gitleaks", args...)
	if timedOut {
		return nil, fmt.Errorf("gitleaks timed out")
	}

	data, readErr := os.ReadFile(reportPath)
	if readErr != nil {
		return nil, fmt.Errorf("cannot read report: %s", firstLine(stderr))
	}

	var leaks []struct {
		Description string `json:"Description"`
		File        string `json:"File"`
		Commit      string `json:"Commit"`
		StartLine   int    `json:"StartLine"`
		EndLine     int    `json:"EndLine"`
		RuleID      string `json:"RuleID"`
		Match       string `json:"Match"`
	}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &leaks); err != nil {
			return nil, fmt.Errorf("failed to parse gitleaks output")
		}
	}

	findings := make([]map[string]any, 0, len(leaks))
	for _, l := range leaks {
		desc := strings.TrimSpace(l.Description)
		if withHistory && l.Commit != "" {
			desc = strings.TrimSpace(desc + " (found in git history)")
		}
		findings = append(findings, map[string]any{
			"rule_id":     l.RuleID,
			"description": desc,
			"path":        relWorkspacePath(base, l.File),
			"start_line":  l.StartLine,
			"end_line":    l.EndLine,
			"match":       redactSecret(l.Match),
		})
	}
	return findings, nil
}

// mergeGitleaksFindings appends history findings, dropping duplicates of the
// working-tree pass keyed by rule + path + line + redacted match.
func mergeGitleaksFindings(base, extra []map[string]any) []map[string]any {
	seen := make(map[string]bool, len(base))
	key := func(f map[string]any) string {
		return fmt.Sprintf("%v|%v|%v|%v", f["rule_id"], f["path"], f["start_line"], f["match"])
	}
	for _, f := range base {
		seen[key(f)] = true
	}
	for _, f := range extra {
		if !seen[key(f)] {
			seen[key(f)] = true
			base = append(base, f)
		}
	}
	return base
}

// exposedTargets are sensitive files/directories whose mere presence in a
// deployable source tree is a finding (credential/source leak if web-served).
var exposedTargets = []struct {
	rel  string
	dir  bool
	desc string
}{
	{".git", true, "Exposed version-control directory (.git): full source and commit history, including any secrets ever committed, are recoverable."},
	{".env", false, "Exposed environment file (.env): typically holds database credentials, API keys and other secrets."},
	{".env.local", false, "Exposed environment file (.env.local) that may contain secrets."},
	{".env.production", false, "Exposed environment file (.env.production) that may contain secrets."},
	{".htpasswd", false, "Exposed .htpasswd file containing credential hashes."},
	{"id_rsa", false, "Exposed private SSH key (id_rsa)."},
	{".aws/credentials", false, "Exposed AWS credentials file."},
}

// detectExposedFiles flags sensitive files/dirs at the repo root and any *.bak /
// *.old / *.swp backup files anywhere in the tree.
func detectExposedFiles(base string) []map[string]any {
	out := make([]map[string]any, 0)

	for _, t := range exposedTargets {
		p := filepath.Join(base, filepath.FromSlash(t.rel))
		info, err := os.Lstat(p)
		if err != nil || info.IsDir() != t.dir {
			continue
		}
		out = append(out, map[string]any{
			"rule_id":     "apant-exposure-" + strings.Trim(strings.ReplaceAll(t.rel, "/", "-"), "."),
			"description": t.desc,
			"path":        filepath.ToSlash(t.rel),
			"start_line":  0,
			"end_line":    0,
			"match":       "",
		})
	}

	// Backup/editor-swap files anywhere in the tree often contain source or creds.
	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != base {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasSuffix(name, ".bak") || strings.HasSuffix(name, ".old") ||
			strings.HasSuffix(name, ".orig") || strings.HasSuffix(name, ".swp") {
			rel, relErr := filepath.Rel(base, path)
			if relErr != nil {
				return nil
			}
			out = append(out, map[string]any{
				"rule_id":     "apant-exposure-backup-file",
				"description": "Exposed backup/temporary file that may contain source code or credentials.",
				"path":        filepath.ToSlash(rel),
				"start_line":  0,
				"end_line":    0,
				"match":       "",
			})
		}
		return nil
	})

	return out
}

// dirExists reports whether p exists and is a directory.
func dirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// vendoredLib describes a front-end library that is commonly committed directly
// into a repo ("vendored") rather than pulled from a package manifest, so
// osv-scanner — which keys off manifests — never sees it. Each entry knows how to
// recognize the library and its version (from the filename or an in-file banner)
// and the version below which it has known CVEs.
type vendoredLib struct {
	name       string
	fileRe     *regexp.Regexp // captures version from the file path
	bannerRe   *regexp.Regexp // captures version from file content
	vulnBelow  [3]int         // versions < this are flagged
	cves       string
	cvssIfVuln string // score string the report maps to a severity
}

var vendoredLibs = []vendoredLib{
	{
		name:       "jquery",
		fileRe:     regexp.MustCompile(`(?i)jquery[-.](\d+\.\d+\.\d+)(?:\.min)?\.js$`),
		bannerRe:   regexp.MustCompile(`(?i)jQuery(?:\s+JavaScript\s+Library)?\s+v?(\d+\.\d+\.\d+)`),
		vulnBelow:  [3]int{3, 5, 0},
		cves:       "CVE-2015-9251, CVE-2019-11358, CVE-2020-11022/11023 (XSS / prototype pollution)",
		cvssIfVuln: "7.5",
	},
	{
		name:       "angular",
		fileRe:     regexp.MustCompile(`(?i)angular(?:js)?[-.](\d+\.\d+\.\d+)(?:\.min)?\.js$`),
		bannerRe:   regexp.MustCompile(`(?i)AngularJS\s+v?(\d+\.\d+\.\d+)`),
		vulnBelow:  [3]int{1, 8, 0},
		cves:       "multiple AngularJS 1.x XSS/sandbox-bypass CVEs",
		cvssIfVuln: "7.5",
	},
	{
		name:       "bootstrap",
		fileRe:     regexp.MustCompile(`(?i)bootstrap[-.](\d+\.\d+\.\d+)(?:\.min)?\.js$`),
		bannerRe:   regexp.MustCompile(`(?i)Bootstrap\s+v?(\d+\.\d+\.\d+)`),
		vulnBelow:  [3]int{4, 3, 1},
		cves:       "CVE-2018-14041, CVE-2019-8331 (XSS in data attributes)",
		cvssIfVuln: "6.1",
	},
	{
		name:       "lodash",
		fileRe:     regexp.MustCompile(`(?i)lodash[-.](\d+\.\d+\.\d+)(?:\.min)?\.js$`),
		bannerRe:   regexp.MustCompile(`(?i)lodash\s+v?(\d+\.\d+\.\d+)`),
		vulnBelow:  [3]int{4, 17, 21},
		cves:       "CVE-2019-10744, CVE-2020-8203 (prototype pollution)",
		cvssIfVuln: "7.4",
	},
}

// detectVendoredComponents scans .js files for known-vulnerable bundled library
// versions (a lightweight retire.js) and returns findings in the osv finding
// shape so they flow through the existing "Vulnerable Dependency" report mapping.
func detectVendoredComponents(base string) []map[string]any {
	out := make([]map[string]any, 0)
	seen := make(map[string]bool) // dedup by lib+version+path

	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != base {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".js") {
			return nil
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		for _, lib := range vendoredLibs {
			version := ""
			if m := lib.fileRe.FindStringSubmatch(relSlash); len(m) == 2 {
				version = m[1]
			} else if data, rerr := os.ReadFile(path); rerr == nil {
				head := data
				if len(head) > 4096 { // version banners live at the top of the file
					head = head[:4096]
				}
				if m := lib.bannerRe.FindStringSubmatch(string(head)); len(m) == 2 {
					version = m[1]
				}
			}
			if version == "" || !versionLess(version, lib.vulnBelow) {
				continue
			}
			key := lib.name + "@" + version + "|" + relSlash
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, map[string]any{
				"id":        "retire-js/" + lib.name,
				"package":   lib.name,
				"version":   version,
				"ecosystem": "JavaScript (vendored)",
				"summary":   fmt.Sprintf("Outdated %s %s bundled in the repository has known vulnerabilities: %s.", lib.name, version, lib.cves),
				"severity":  lib.cvssIfVuln,
				"path":      relSlash,
			})
		}
		return nil
	})

	return out
}

// misconfigRule is a low-severity security misconfiguration detectable by a
// simple line pattern in config/source files (debug flags, verbose errors, info
// disclosure). These populate the Low/Informational severity bands.
type misconfigRule struct {
	ruleID  string
	re      *regexp.Regexp
	message string
}

var misconfigRules = []misconfigRule{
	{
		"apant-debug-mode-enabled",
		regexp.MustCompile(`(?i)\bAPP_DEBUG\s*=\s*true\b|\bAPP_ENV\s*=\s*(?:local|development)\b|app\.debug\s*=\s*true`),
		"Debug mode appears enabled. In production this leaks stack traces, queries and internal details to users.",
	},
	{
		"apant-display-errors-on",
		regexp.MustCompile(`(?i)\bdisplay_errors\s*=\s*(?:on|1|true)\b|ini_set\(\s*['"]display_errors['"]\s*,\s*['"]?(?:1|on|true)`),
		"Verbose error display appears enabled (display_errors). This can disclose paths, SQL and internal state to attackers.",
	},
	{
		"apant-phpinfo-call",
		regexp.MustCompile(`(?i)\bphpinfo\s*\(`),
		"phpinfo() call detected. If reachable it discloses the full PHP configuration and environment (information disclosure).",
	},
}

// detectMisconfigurations scans config/source files for low-severity security
// misconfigurations (debug/verbose modes, info disclosure). Findings use the
// semgrep finding shape so they flow through the existing report mapping.
func detectMisconfigurations(base string) []map[string]any {
	out := make([]map[string]any, 0)

	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != base {
				return filepath.SkipDir
			}
			return nil
		}
		if !isConfigOrCodeFile(d.Name()) {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil || len(data) > maxReadFileBytes {
			return nil
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		for i, line := range strings.Split(string(data), "\n") {
			for _, r := range misconfigRules {
				if r.re.MatchString(line) {
					out = append(out, map[string]any{
						"rule_id":    r.ruleID,
						"path":       relSlash,
						"start_line": i + 1,
						"end_line":   i + 1,
						"severity":   "info",
						"message":    r.message,
						"snippet":    clampSnippet(strings.TrimSpace(line)),
						"confidence": "high",
					})
				}
			}
			if len(out) >= maxSemgrepFindings {
				return filepath.SkipAll
			}
		}
		return nil
	})

	return out
}

// isConfigOrCodeFile reports whether a filename is worth scanning for
// misconfigurations (env/config files and PHP/JS source).
func isConfigOrCodeFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, ".env") {
		return true
	}
	for _, suf := range []string{".php", ".js", ".ini", ".conf", ".cfg", ".config", ".env", ".yaml", ".yml"} {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	return false
}

// versionLess reports whether the dotted version string is lower than ref.
func versionLess(version string, ref [3]int) bool {
	var v [3]int
	parts := strings.SplitN(version, ".", 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		n := 0
		fmt.Sscanf(parts[i], "%d", &n)
		v[i] = n
	}
	for i := 0; i < 3; i++ {
		if v[i] != ref[i] {
			return v[i] < ref[i]
		}
	}
	return false // equal is not "less"
}

func (e *MultiExecutor) executeOSVScan(intent *domain.ToolIntent) map[string]any {
	base, err := e.resolveWorkspace(intent)
	if err != nil {
		return staticErr("osv_scan", err.Error())
	}

	// Vendored front-end libraries (e.g. jquery-1.7.2.min.js) have no manifest, so
	// osv-scanner cannot see them; detect them separately and always include them.
	vendored := detectVendoredComponents(base)

	stdout, stderr, _, timedOut := e.runStaticCmd("osv-scanner", "--format", "json", "--recursive", base)
	if timedOut {
		return map[string]any{"status": "error", "tool": "osv_scan", "error": "osv_scan timed out"}
	}
	if strings.TrimSpace(stdout) == "" {
		// No manifests or no connectivity — still report any vendored components.
		return map[string]any{
			"status": "success", "tool": "osv_scan", "count": len(vendored),
			"findings": vendored, "note": firstLine(stderr),
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

	findings = append(findings, vendored...)
	if len(findings) > maxOSVFindings {
		findings = findings[:maxOSVFindings]
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
