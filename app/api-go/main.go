package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
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

type CollectionInitRequest struct {
	Name       string `json:"name"`
	VectorSize int    `json:"vector_size"`
	Distance   string `json:"distance"`
}

type QdrantCreateCollectionRequest struct {
	Vectors QdrantVectorsConfig `json:"vectors"`
}

type QdrantVectorsConfig struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

func main() {
	port := getEnv("PORT", "8000")
	qdrantURL := getEnv("QDRANT_URL", "http://qdrant:6333")

	app := &App{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		qdrantURL: qdrantURL,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", app.handleRoot)
	mux.HandleFunc("/health", app.handleHealth)
	mux.HandleFunc("/ready", app.handleReady)
	mux.HandleFunc("/collections", app.handleCollections)
	mux.HandleFunc("/collections/init", app.handleCollectionsInit)

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
		"endpoints":   "/health, /ready, /collections, /collections/init",
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Service: "api-go",
	})
}

func (a *App) handleReady(w http.ResponseWriter, r *http.Request) {
	resp, body, err := a.doQdrantRequest(http.MethodGet, "/", nil)
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
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error: "method not allowed",
		})
		return
	}

	resp, body, err := a.doQdrantRequest(http.MethodGet, "/collections", nil)
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

func (a *App) handleCollectionsInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error: "method not allowed",
		})
		return
	}

	var req CollectionInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid json body",
		})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "name is required",
		})
		return
	}

	if req.VectorSize == 0 {
		req.VectorSize = 768
	}

	distance, err := normalizeDistance(req.Distance)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	qdrantReq := QdrantCreateCollectionRequest{
		Vectors: QdrantVectorsConfig{
			Size:     req.VectorSize,
			Distance: distance,
		},
	}

	resp, body, err := a.doQdrantRequest(
		http.MethodPut,
		"/collections/"+url.PathEscape(req.Name),
		qdrantReq,
	)
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

func (a *App) doQdrantRequest(method, path string, payload any) (*http.Response, []byte, error) {
	var bodyReader io.Reader

	if payload != nil {
		requestBody, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal qdrant request: %w", err)
		}
		bodyReader = bytes.NewReader(requestBody)
	}

	req, err := http.NewRequest(method, a.qdrantURL+path, bodyReader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request to qdrant: %w", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
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

func normalizeDistance(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "Cosine", nil
	}

	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cosine":
		return "Cosine", nil
	case "dot":
		return "Dot", nil
	case "euclid":
		return "Euclid", nil
	case "manhattan":
		return "Manhattan", nil
	default:
		return "", fmt.Errorf("unsupported distance: %s", value)
	}
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
