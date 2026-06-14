package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// GotenbergConverter renders HTML to PDF via a Gotenberg service (Chromium-based)
// over HTTP. It implements domain.PDFConverter and keeps the api image light by
// delegating rendering to a separate, swappable service.
type GotenbergConverter struct {
	baseURL string
	client  *http.Client
}

func NewGotenbergConverter(baseURL string) *GotenbergConverter {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "http://gotenberg:3000"
	}

	return &GotenbergConverter{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (g *GotenbergConverter) HTMLToPDF(ctx context.Context, html []byte) ([]byte, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	// Gotenberg's Chromium HTML route requires the entry document to be named
	// index.html; the form field name itself is arbitrary.
	part, err := w.CreateFormFile("files", "index.html")
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(html); err != nil {
		return nil, fmt.Errorf("write html: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	url := g.baseURL + "/forms/chromium/convert/html"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request failed: %w", err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read gotenberg response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gotenberg error %d: %s", resp.StatusCode, strings.TrimSpace(string(out)))
	}

	return out, nil
}
