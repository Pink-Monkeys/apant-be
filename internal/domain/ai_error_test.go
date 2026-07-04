package domain

import "testing"

func TestNewProviderErrorExtractsMessage(t *testing.T) {
	t.Run("openrouter metadata.raw preferred", func(t *testing.T) {
		body := []byte(`{"error":{"message":"Provider returned error","code":429,"metadata":{"raw":"model X is temporarily rate-limited upstream. Please retry shortly"}}}`)
		e := NewProviderError(429, body)
		if e.StatusCode != 429 {
			t.Fatalf("status = %d, want 429", e.StatusCode)
		}
		if e.Message != "model X is temporarily rate-limited upstream. Please retry shortly" {
			t.Fatalf("message = %q", e.Message)
		}
	})

	t.Run("falls back to error.message", func(t *testing.T) {
		body := []byte(`{"error":{"message":"invalid api key"}}`)
		e := NewProviderError(401, body)
		if e.Message != "invalid api key" {
			t.Fatalf("message = %q", e.Message)
		}
	})

	t.Run("falls back to raw body when not JSON", func(t *testing.T) {
		e := NewProviderError(500, []byte("upstream exploded"))
		if e.Message != "upstream exploded" {
			t.Fatalf("message = %q", e.Message)
		}
	})

	t.Run("Error() never empty", func(t *testing.T) {
		e := NewProviderError(503, []byte(""))
		if e.Error() == "" {
			t.Fatal("Error() should not be empty")
		}
	})
}
