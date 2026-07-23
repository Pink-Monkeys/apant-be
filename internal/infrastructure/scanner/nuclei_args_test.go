package scanner

import (
	"strings"
	"testing"
	"time"

	"apant_be/internal/domain"
)

// contains reports whether flag appears immediately followed by value in args.
func hasFlagValue(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// TestBuildNucleiArgs_WordPressTags locks the Step 0 "V1" decision: a tags run filters
// by tag with NO -t path restriction (the full tree is loaded, only tagged templates
// execute). Regression guard against reintroducing hardcoded -t cves/ scoping.
func TestBuildNucleiArgs_WordPressTags(t *testing.T) {
	e := NewMultiExecutor(MultiExecutorConfig{})
	intent := &domain.ToolIntent{
		Name:   "nuclei_scan",
		Params: map[string]any{"target": "http://wp.example", "tags": "wordpress"},
	}

	args, err := e.buildNucleiArgs(intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasFlagValue(args, "-tags", "wordpress") {
		t.Fatalf("expected -tags wordpress, got %v", args)
	}
	if hasFlag(args, "-t") {
		t.Fatalf("tags run must NOT pass any -t path, got %v", args)
	}
}

// TestBuildNucleiArgs_TagsBeatTemplates ensures the tags branch takes precedence over
// an agent-supplied templates path, so a tags run is never silently narrowed by a -t.
func TestBuildNucleiArgs_TagsBeatTemplates(t *testing.T) {
	e := NewMultiExecutor(MultiExecutorConfig{})
	intent := &domain.ToolIntent{
		Name: "nuclei_scan",
		Params: map[string]any{
			"target":    "http://wp.example",
			"tags":      "wordpress",
			"templates": "/home/scanner/.nuclei-templates/http/cves/",
		},
	}

	args, err := e.buildNucleiArgs(intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasFlagValue(args, "-tags", "wordpress") || hasFlag(args, "-t") {
		t.Fatalf("tags must win over templates, got %v", args)
	}
}

// TestBuildNucleiArgs_DefaultCurated confirms the untouched default path still scopes
// to curated -t categories and adds no -tags.
func TestBuildNucleiArgs_DefaultCurated(t *testing.T) {
	e := NewMultiExecutor(MultiExecutorConfig{})
	intent := &domain.ToolIntent{
		Name:   "nuclei_scan",
		Params: map[string]any{"target": "http://x.example"},
	}

	args, err := e.buildNucleiArgs(intent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasFlag(args, "-tags") {
		t.Fatalf("default run must not add -tags, got %v", args)
	}
	if !hasFlag(args, "-t") {
		t.Fatalf("default run must scope with -t categories, got %v", args)
	}
}

// TestNucleiTimeout covers the layer-2 fix: a WP tags run widens the exec ceiling; a
// plain run keeps the base; an already-wider base is preserved.
func TestNucleiTimeout(t *testing.T) {
	base := 300 * time.Second
	cases := []struct {
		name   string
		base   time.Duration
		params map[string]any
		want   time.Duration
	}{
		{"wp tags widens", base, map[string]any{"tags": "wordpress"}, wpNucleiTimeout},
		{"no tags keeps base", base, map[string]any{}, base},
		{"blank tags keeps base", base, map[string]any{"tags": "  "}, base},
		{"wider base preserved", 900 * time.Second, map[string]any{"tags": "wordpress"}, 900 * time.Second},
	}
	for _, tc := range cases {
		if got := nucleiTimeout(tc.base, tc.params); got != tc.want {
			t.Errorf("%s: nucleiTimeout=%v want %v", tc.name, got, tc.want)
		}
	}
}

// TestValidateNucleiParams_TagAllowlist enforces that only curated tags pass, so the
// LLM cannot request a broad tag that would execute thousands of templates.
func TestValidateNucleiParams_TagAllowlist(t *testing.T) {
	cases := []struct {
		tags    string
		wantErr bool
	}{
		{"wordpress", false},
		{"WordPress", false},
		{"cve", true},
		{"wordpress,drupal", true},
		{"", false},
	}
	for _, tc := range cases {
		params := map[string]any{"tags": tc.tags}
		if strings.TrimSpace(tc.tags) == "" {
			params = map[string]any{}
		}
		err := validateNucleiParams(params)
		if tc.wantErr && err == nil {
			t.Errorf("tags=%q: expected error, got nil", tc.tags)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("tags=%q: unexpected error %v", tc.tags, err)
		}
	}
}
