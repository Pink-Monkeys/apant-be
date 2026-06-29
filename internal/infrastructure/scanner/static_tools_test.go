package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"apant_be/internal/domain"
)

func TestIsWithin(t *testing.T) {
	root := filepath.FromSlash("/workspace/scan/src")
	cases := []struct {
		path string
		want bool
	}{
		{filepath.FromSlash("/workspace/scan/src"), true},
		{filepath.FromSlash("/workspace/scan/src/app/main.go"), true},
		{filepath.FromSlash("/workspace/scan/other"), false},
		{filepath.FromSlash("/workspace"), false},
		{filepath.FromSlash("/etc/passwd"), false},
	}
	for _, c := range cases {
		if got := isWithin(root, c.path); got != c.want {
			t.Errorf("isWithin(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestResolveInside(t *testing.T) {
	base := filepath.FromSlash("/workspace/scan/src")

	ok, err := resolveInside(base, "app/main.go")
	if err != nil {
		t.Fatalf("unexpected error for valid path: %v", err)
	}
	if ok != filepath.Join(base, "app", "main.go") {
		t.Fatalf("unexpected resolved path: %s", ok)
	}

	for _, bad := range []string{"../secret", "../../etc/passwd", "app/../../escape"} {
		if _, err := resolveInside(base, bad); err == nil {
			t.Errorf("expected traversal rejection for %q", bad)
		}
	}

	// Use a path that is absolute on the host OS (on Windows a bare "/etc" lacks a
	// drive letter and is treated as an in-workspace subdir, which is itself safe).
	abs, _ := filepath.Abs(filepath.Join(string(filepath.Separator), "etc", "passwd"))
	if _, err := resolveInside(base, abs); err == nil {
		t.Error("expected absolute path rejection")
	}
}

func TestDetectExposedFiles(t *testing.T) {
	base := t.TempDir()

	// Sensitive files/dirs that must be flagged.
	mustWrite(t, filepath.Join(base, ".env"), "DB_PASS=secret")
	mustMkdir(t, filepath.Join(base, ".git"))
	mustWrite(t, filepath.Join(base, "config.php.bak"), "<?php $pass='x';")
	mustWrite(t, filepath.Join(base, "src", "app.old"), "legacy")
	// Benign files that must NOT be flagged.
	mustWrite(t, filepath.Join(base, "index.php"), "<?php echo 1;")
	mustWrite(t, filepath.Join(base, "README.md"), "docs")

	found := map[string]bool{}
	for _, f := range detectExposedFiles(base) {
		found[f["path"].(string)] = true
	}

	for _, want := range []string{".env", ".git", "config.php.bak", "src/app.old"} {
		if !found[want] {
			t.Errorf("expected exposed-file finding for %q, got %v", want, found)
		}
	}
	for _, notWant := range []string{"index.php", "README.md"} {
		if found[notWant] {
			t.Errorf("did not expect a finding for benign file %q", notWant)
		}
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDetectVendoredComponents(t *testing.T) {
	base := t.TempDir()

	// Vulnerable jQuery by filename + banner.
	mustWrite(t, filepath.Join(base, "app", "assets", "jquery-1.7.2.min.js"),
		"/*! jQuery v1.7.2 jquery.com */\nwindow.jQuery={};")
	// Up-to-date jQuery: must NOT be flagged.
	mustWrite(t, filepath.Join(base, "app", "assets", "jquery-3.6.0.min.js"),
		"/*! jQuery v3.6.0 */\n")
	// Vulnerable library detected only by in-file banner (generic filename).
	mustWrite(t, filepath.Join(base, "vendor-bootstrap.js"),
		"/*! Bootstrap v3.3.7 */\n")
	// Unrelated JS: no finding.
	mustWrite(t, filepath.Join(base, "app.js"), "console.log('hi');")

	byPkgVer := map[string]string{}
	for _, f := range detectVendoredComponents(base) {
		byPkgVer[f["package"].(string)+"@"+f["version"].(string)] = f["path"].(string)
	}

	if _, ok := byPkgVer["jquery@1.7.2"]; !ok {
		t.Errorf("expected jquery@1.7.2 flagged, got %v", byPkgVer)
	}
	if _, ok := byPkgVer["bootstrap@3.3.7"]; !ok {
		t.Errorf("expected bootstrap@3.3.7 flagged via banner, got %v", byPkgVer)
	}
	if _, ok := byPkgVer["jquery@3.6.0"]; ok {
		t.Error("jquery@3.6.0 is current and must NOT be flagged")
	}
}

func TestDetectMisconfigurations(t *testing.T) {
	base := t.TempDir()

	mustWrite(t, filepath.Join(base, ".env"), "APP_ENV=production\nAPP_DEBUG=true\nDB_PASS=x")
	mustWrite(t, filepath.Join(base, "index.php"), "<?php\nini_set('display_errors', 1);\nphpinfo();\n")
	mustWrite(t, filepath.Join(base, "safe.php"), "<?php echo 'ok';")

	rules := map[string]bool{}
	for _, f := range detectMisconfigurations(base) {
		rules[f["rule_id"].(string)] = true
		if f["severity"].(string) != "info" {
			t.Errorf("expected info severity, got %v", f["severity"])
		}
	}

	for _, want := range []string{"apant-debug-mode-enabled", "apant-display-errors-on", "apant-phpinfo-call"} {
		if !rules[want] {
			t.Errorf("expected misconfig rule %q to fire, got %v", want, rules)
		}
	}
}

func TestVersionLess(t *testing.T) {
	cases := []struct {
		v    string
		ref  [3]int
		want bool
	}{
		{"1.7.2", [3]int{3, 5, 0}, true},
		{"3.5.0", [3]int{3, 5, 0}, false}, // equal is not less
		{"3.6.0", [3]int{3, 5, 0}, false},
		{"3.4.99", [3]int{3, 5, 0}, true},
		{"2", [3]int{3, 0, 0}, true},
	}
	for _, c := range cases {
		if got := versionLess(c.v, c.ref); got != c.want {
			t.Errorf("versionLess(%q,%v) = %v, want %v", c.v, c.ref, got, c.want)
		}
	}
}

func TestResolveWorkspaceRejectsBadScanID(t *testing.T) {
	e := NewMultiExecutor(MultiExecutorConfig{WorkspaceRoot: filepath.FromSlash("/workspace")})
	for _, bad := range []string{"", "../etc", "a/b", "foo/../bar", "name with space"} {
		intent := &domain.ToolIntent{Name: "read_file", Params: map[string]any{"scan_id": bad}}
		if _, err := e.resolveWorkspace(intent); err == nil {
			t.Errorf("expected rejection for scan_id %q", bad)
		}
	}
}
