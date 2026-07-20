package domain

import (
	"context"
	"sync"
)

// TokenTally accumulates LLM token usage across every model call made during one
// logical operation (e.g. a single scan run). It is carried on the request
// context so the gateway can add each call's usage without threading a counter
// through every call site. Safe for concurrent use; within one scan the calls are
// sequential, but the mutex keeps it correct if that ever changes.
type TokenTally struct {
	mu           sync.Mutex
	inputTokens  int
	outputTokens int
	totalTokens  int
	calls        int
}

// Add folds one provider call's usage into the tally. A total of 0 means the
// provider did not report one, so it is derived from in+out. A call with all-zero
// usage still counts toward Calls (so "provider reported no usage" is visible).
func (t *TokenTally) Add(in, out, total int) {
	if t == nil {
		return
	}
	if total <= 0 {
		total = in + out
	}
	t.mu.Lock()
	t.inputTokens += in
	t.outputTokens += out
	t.totalTokens += total
	t.calls++
	t.mu.Unlock()
}

// TokenSnapshot is an immutable read of a TokenTally at a point in time.
type TokenSnapshot struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	Calls        int
}

// Snapshot returns the accumulated totals. Nil-safe (returns a zero snapshot).
func (t *TokenTally) Snapshot() TokenSnapshot {
	if t == nil {
		return TokenSnapshot{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return TokenSnapshot{
		InputTokens:  t.inputTokens,
		OutputTokens: t.outputTokens,
		TotalTokens:  t.totalTokens,
		Calls:        t.calls,
	}
}

type tokenTallyCtxKey struct{}

// ContextWithTokenTally returns a context carrying a TokenTally plus the tally
// itself. If the context already carries one, it is reused (so nested scopes
// accumulate into the same tally rather than starting fresh).
func ContextWithTokenTally(ctx context.Context) (context.Context, *TokenTally) {
	if existing, ok := ctx.Value(tokenTallyCtxKey{}).(*TokenTally); ok && existing != nil {
		return ctx, existing
	}
	t := &TokenTally{}
	return context.WithValue(ctx, tokenTallyCtxKey{}, t), t
}

// TokenTallyFromContext returns the tally on the context, or nil if none was
// installed (in which case Add is a no-op and nothing is counted).
func TokenTallyFromContext(ctx context.Context) *TokenTally {
	t, _ := ctx.Value(tokenTallyCtxKey{}).(*TokenTally)
	return t
}
