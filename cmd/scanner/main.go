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
	timeout := time.Duration(getEnvInt("NMAP_TIMEOUT_SECONDS", 60)) * time.Second

	executor := scanner.NewLocalNmapExecutor(scanner.LocalNmapConfig{
		Binary:  nmapBinary,
		Timeout: timeout,
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
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
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
