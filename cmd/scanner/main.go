package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"apant_be/internal/domain"
	"apant_be/internal/infrastructure/scanner"
)

type responseWriter struct {
	http.ResponseWriter
}

func main() {
	port := getEnv("SCANNER_PORT", "8081")
	nmapBinary := getEnv("NMAP_BINARY", "nmap")

	nmapTimeout := time.Duration(getEnvInt("NMAP_TIMEOUT_SECONDS", 60)) * time.Second
	toolTimeout := time.Duration(getEnvInt("SCANNER_TOOL_TIMEOUT_SECONDS", 300)) * time.Second

	executor := scanner.NewMultiExecutor(scanner.MultiExecutorConfig{
		NmapBinary:      nmapBinary,
		NmapTimeout:     nmapTimeout,
		Timeout:         toolTimeout,
		NucleiTemplates: getEnv("NUCLEI_TEMPLATES_PATH", "/home/scanner/.nuclei-templates"),
		WordlistsDir:    "/wordlists",
		WorkspaceRoot:   getEnv("SCANNER_WORKSPACE_DIR", "/workspace"),
		SemgrepConfig:   getEnv("SEMGREP_CONFIG", "/opt/semgrep-rules"),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "method not allowed"})
			return
		}

		var intent domain.ToolIntent
		if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid json"})
			return
		}

		intent.Name = strings.TrimSpace(strings.ToLower(intent.Name))
		if intent.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "tool name is required"})
			return
		}

		result := executor.Execute(&intent)
		status := http.StatusOK
		if v, ok := result["status"].(string); ok && v == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, result)
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Must exceed the longest per-tool exec ceiling so a long-running tool's
		// response is flushed rather than cut mid-write. The WordPress/CMS nuclei
		// path (-tags) raises its exec ceiling to 600s, so this stays above that.
		WriteTimeout: 660 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("scanner service listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("scanner service error: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
