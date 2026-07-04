package ai

import "context"

// ProbeAdapter adapts ProbeProvider to the llm.Prober interface, keeping the
// application layer free of infrastructure imports.
type ProbeAdapter struct{}

func NewProbeAdapter() ProbeAdapter { return ProbeAdapter{} }

func (ProbeAdapter) Probe(ctx context.Context, adapterType, apiKey, baseURL string) (bool, []string, error) {
	res, err := ProbeProvider(ctx, adapterType, apiKey, baseURL)
	return res.OK, res.Models, err
}
