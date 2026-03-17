package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type App struct {
	httpClient *http.Client
	qdrantURL  string
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type ReadyResponse struct {
	Status    string          `json:"status"`
	Service   string          `json:"service"`
	QdrantURL string          `json:"qdrant_url"`
	Qdrant    json.RawMessage `json:"qdrant,omitempty"`
	Error     string          `json:"error,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	port := getEnv("PORT", "8000")
	qdrantURL := getEnv("QDRANT_URL", "http://qdrant:6333")

	app := &App{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		qdrantURL: qdrantURL,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", app.handleRoot)
	mux.HandleFunc("/health", app.handleHealth)
	mux.HandleFunc("/ready", app.handleReady)
	mux.HandleFunc("/collections", app.handleCollections)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("starting api-go on port %s", port)
	log.Printf("using qdrant url: %s", qdrantURL)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error: "route not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"service":     "api-go",
		"status":      "ok",
		"description": "AI inference platform API",
		"endpoints":   "/health, /ready, /collections",
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Service: "api-go",
	})
}

func (a *App) handleReady(w http.ResponseWriter, r *http.Request) {
	resp, body, err := a.fetchQdrant("/")
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, ReadyResponse{
			Status:    "not ready",
			Service:   "api-go",
			QdrantURL: a.qdrantURL,
			Error:     err.Error(),
		})
		return
	}

	if resp.StatusCode >= 400 {
		writeJSON(w, http.StatusServiceUnavailable, ReadyResponse{
			Status:    "not ready",
			Service:   "api-go",
			QdrantURL: a.qdrantURL,
			Error:     fmt.Sprintf("qdrant returned status %d", resp.StatusCode),
		})
		return
	}

	writeJSON(w, http.StatusOK, ReadyResponse{
		Status:    "ready",
		Service:   "api-go",
		QdrantURL: a.qdrantURL,
		Qdrant:    body,
	})
}

func (a *App) handleCollections(w http.ResponseWriter, r *http.Request) {
	resp, body, err := a.fetchQdrant("/collections")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

func (a *App) fetchQdrant(path string) (*http.Response, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, a.qdrantURL+path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request to qdrant: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to reach qdrant: %w", err)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		return resp, nil, fmt.Errorf("failed to read qdrant response: %w", err)
	}

	return resp, body, nil
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer func() {
		_ = resp.Body.Close()
	}()

	return io.ReadAll(resp.Body)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(startedAt))
	})
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
