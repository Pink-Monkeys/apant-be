package ai

import (
	"context"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Transient-failure retry policy for LLM provider calls. A single 429/500/timeout
// from the upstream must NOT kill an entire scan (25+ sequential steps): one
// dropped step aborts everything the agent has built. These bounds keep a stalled
// provider from stretching a scan indefinitely while still riding out the common
// blips (rate limits, brief 5xx, connection resets).
const (
	maxRetryAttempts = 4                      // 1 initial try + 3 retries
	retryBaseDelay   = 500 * time.Millisecond // first backoff; doubles each retry
	retryMaxDelay    = 8 * time.Second        // cap for exponential/Retry-After waits
)

// doJSONWithRetry performs an HTTP request with bounded exponential backoff,
// retrying only transient failures: network/timeout errors, HTTP 429, and 5xx.
// Permanent failures (4xx other than 429 — auth, bad request) are returned
// immediately without wasting retries. newReq builds a fresh *http.Request on
// every attempt because the request body reader is consumed once per send.
//
// It returns the final response's status code and fully-read body, or a non-nil
// error when every attempt failed to get a response (or the context ended). The
// caller keeps its existing status handling: a returned status >= 300 is a
// non-retryable (or retry-exhausted) provider error to be wrapped as before.
func doJSONWithRetry(
	ctx context.Context,
	client *http.Client,
	label string,
	newReq func() (*http.Request, error),
) (int, []byte, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
		req, err := newReq()
		if err != nil {
			return 0, nil, err // request construction is deterministic; retrying cannot help
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return 0, nil, ctx.Err() // caller cancelled/timed out; stop retrying
			}
			if attempt < maxRetryAttempts {
				log.Printf("%s: request failed (attempt %d/%d): %v", label, attempt, maxRetryAttempts, err)
				if waitErr := backoffWait(ctx, attempt, 0); waitErr != nil {
					return 0, nil, waitErr
				}
				continue
			}
			return 0, nil, err
		}

		body, readErr := io.ReadAll(resp.Body)
		retryAfter := resp.Header.Get("Retry-After")
		status := resp.StatusCode
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < maxRetryAttempts {
				log.Printf("%s: read body failed (attempt %d/%d): %v", label, attempt, maxRetryAttempts, readErr)
				if waitErr := backoffWait(ctx, attempt, 0); waitErr != nil {
					return 0, nil, waitErr
				}
				continue
			}
			return 0, nil, readErr
		}

		if isTransientStatus(status) && attempt < maxRetryAttempts {
			log.Printf("%s: transient HTTP %d (attempt %d/%d), backing off", label, status, attempt, maxRetryAttempts)
			if waitErr := backoffWait(ctx, attempt, parseRetryAfter(retryAfter)); waitErr != nil {
				return 0, nil, waitErr
			}
			continue
		}

		return status, body, nil
	}

	return 0, nil, lastErr
}

// isTransientStatus reports whether an HTTP status is worth retrying: 429 (rate
// limited) and any 5xx (upstream/server error). Everything else — including 4xx
// like 401/403/400 — is a caller-side problem that a retry only makes worse.
func isTransientStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// backoffWait sleeps before the next attempt: it honors a server-provided
// Retry-After hint when present, otherwise uses exponential backoff with jitter.
// It returns early with ctx.Err() if the context ends during the wait, so a
// cancelled scan never blocks on a sleep.
func backoffWait(ctx context.Context, attempt int, retryAfter time.Duration) error {
	delay := retryAfter
	if delay <= 0 {
		// Exponential: base * 2^(attempt-1), plus up to 250ms jitter to avoid
		// synchronized retries hammering the provider in lockstep.
		delay = retryBaseDelay * time.Duration(1<<(attempt-1))
		delay += time.Duration(rand.Int63n(int64(250 * time.Millisecond)))
	}
	if delay > retryMaxDelay {
		delay = retryMaxDelay
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// parseRetryAfter interprets a Retry-After header value. Providers send either a
// number of seconds ("5") or an HTTP date; only the numeric form is honored (the
// common rate-limit case), capped by the caller. Anything else yields 0, letting
// backoffWait fall back to exponential timing.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
