package ai

import (
	"testing"

	"apant_be/internal/domain"
)

func TestBuildProvider(t *testing.T) {
	cases := []struct {
		adapter  string
		wantName string
		wantErr  bool
	}{
		{domain.AdapterOpenAICompatible, "openai", false},
		{domain.AdapterOpenAIChat, "openai-chat", false},
		{domain.AdapterAnthropic, "claude", false},
		{"mystery", "", true},
	}
	for _, c := range cases {
		t.Run(c.adapter, func(t *testing.T) {
			p, err := BuildProvider(c.adapter, "key", "https://example.com", "some-model")
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for adapter %q", c.adapter)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Name() != c.wantName {
				t.Fatalf("adapter %q → provider %q, want %q", c.adapter, p.Name(), c.wantName)
			}
		})
	}
}
