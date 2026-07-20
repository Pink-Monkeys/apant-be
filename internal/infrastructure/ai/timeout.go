package ai

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// defaultProviderTimeout is the per-call HTTP timeout for LLM providers. It must
// cover the whole non-streaming completion: the server generates the entire
// answer before sending response headers, so a large agentic prompt on a slow or
// reasoning model can legitimately take well over a minute. 60s was too tight and
// produced "Client.Timeout exceeded while awaiting headers".
const defaultProviderTimeout = 120 * time.Second

// providerHTTPTimeout is the per-call timeout applied to every LLM provider
// adapter. Override with LLM_HTTP_TIMEOUT_SECONDS (whole seconds) when using a
// slow model/endpoint; values <= 0 or unparseable fall back to the default.
//
// Note: doJSONWithRetry retries a timed-out call, so the worst-case wait for one
// step is roughly this value times the retry attempts — raise it deliberately.
func providerHTTPTimeout() time.Duration {
	v := strings.TrimSpace(os.Getenv("LLM_HTTP_TIMEOUT_SECONDS"))
	if v == "" {
		return defaultProviderTimeout
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs <= 0 {
		return defaultProviderTimeout
	}
	return time.Duration(secs) * time.Second
}
