package scanner

import (
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

func TestResolveWorkspaceRejectsBadScanID(t *testing.T) {
	e := NewMultiExecutor(MultiExecutorConfig{WorkspaceRoot: filepath.FromSlash("/workspace")})
	for _, bad := range []string{"", "../etc", "a/b", "foo/../bar", "name with space"} {
		intent := &domain.ToolIntent{Name: "read_file", Params: map[string]any{"scan_id": bad}}
		if _, err := e.resolveWorkspace(intent); err == nil {
			t.Errorf("expected rejection for scan_id %q", bad)
		}
	}
}
