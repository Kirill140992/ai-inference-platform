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
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

var (
	apiRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "Duration of HTTP requests handled by api-go.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	qdrantRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "qdrant_request_duration_seconds",
			Help:    "Duration of outbound requests from api-go to Qdrant.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	qdrantRequestErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "qdrant_request_errors_total",
			Help: "Total number of outbound request errors from api-go to Qdrant.",
		},
		[]string{"method", "path"},
	)

	apiReadinessFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "api_readiness_failures_total",
			Help: "Total number of readiness failures in api-go.",
		},
	)
)

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

	mux.Handle("/", app.instrumentHandler("/", http.HandlerFunc(app.handleRoot)))
	mux.Handle("/health", app.instrumentHandler("/health", http.HandlerFunc(app.handleHealth)))
	mux.Handle("/ready", app.instrumentHandler("/ready", http.HandlerFunc(app.handleReady)))
	mux.Handle("/collections", app.instrumentHandler("/collections", http.HandlerFunc(app.handleCollections)))
	mux.Handle("/collections/init", app.instrumentHandler("/collections/init", http.HandlerFunc(app.handleCollectionsInit)))
	mux.Handle("/metrics", promhttp.Handler())

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

func (a *App) instrumentHandler(path string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()

		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		apiRequestDuration.WithLabelValues(
			r.Method,
			path,
			strconv.Itoa(recorder.statusCode),
		).Observe(time.Since(startedAt).Seconds())
	})
}

func (sr *statusRecorder) WriteHeader(statusCode int) {
	sr.statusCode = statusCode
	sr.ResponseWriter.WriteHeader(statusCode)
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
		"endpoints":   "/health, /ready, /collections, /collections/init, /metrics",
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
		apiReadinessFailuresTotal.Inc()
		writeJSON(w, http.StatusServiceUnavailable, ReadyResponse{
			Status:    "not ready",
			Service:   "api-go",
			QdrantURL: a.qdrantURL,
			Error:     err.Error(),
		})
		return
	}

	if resp.StatusCode >= 400 {
		apiReadinessFailuresTotal.Inc()
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
	startedAt := time.Now()

	var bodyReader io.Reader

	if payload != nil {
		requestBody, err := json.Marshal(payload)
		if err != nil {
			qdrantRequestErrorsTotal.WithLabelValues(method, path).Inc()
			return nil, nil, fmt.Errorf("failed to marshal qdrant request: %w", err)
		}
		bodyReader = bytes.NewReader(requestBody)
	}

	req, err := http.NewRequest(method, a.qdrantURL+path, bodyReader)
	if err != nil {
		qdrantRequestErrorsTotal.WithLabelValues(method, path).Inc()
		return nil, nil, fmt.Errorf("failed to create request to qdrant: %w", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		qdrantRequestErrorsTotal.WithLabelValues(method, path).Inc()
		return nil, nil, fmt.Errorf("failed to reach qdrant: %w", err)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		qdrantRequestErrorsTotal.WithLabelValues(method, path).Inc()
		return resp, nil, fmt.Errorf("failed to read qdrant response: %w", err)
	}

	qdrantRequestDuration.WithLabelValues(
		method,
		path,
		strconv.Itoa(resp.StatusCode),
	).Observe(time.Since(startedAt).Seconds())

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
