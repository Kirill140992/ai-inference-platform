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

type DocumentIngestRequest struct {
	DocumentID   string `json:"document_id"`
	Title        string `json:"title"`
	Source       string `json:"source"`
	Content      string `json:"content"`
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap int    `json:"chunk_overlap"`
	DryRun       bool   `json:"dry_run"`
}

type DocumentIngestResponse struct {
	Status       string          `json:"status"`
	DocumentID   string          `json:"document_id"`
	Title        string          `json:"title"`
	Source       string          `json:"source"`
	ChunkSize    int             `json:"chunk_size"`
	ChunkOverlap int             `json:"chunk_overlap"`
	ChunkCount   int             `json:"chunk_count"`
	ChunkingMode string          `json:"chunking_mode"`
	ChunkingVer  string          `json:"chunking_version"`
	DryRun       bool            `json:"dry_run"`
	Chunks       []DocumentChunk `json:"chunks"`
}

type DocumentChunk struct {
	ChunkID    string                 `json:"chunk_id"`
	ChunkIndex int                    `json:"chunk_index"`
	Text       string                 `json:"text"`
	Metadata   map[string]interface{} `json:"metadata"`
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

const chunkingVersion = "chunker_v1"

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

	documentChunksProducedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "document_chunks_produced_total",
			Help: "Total number of document chunks produced by the ingestion endpoint.",
		},
		[]string{"document_id"},
	)

	documentIngestRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "document_ingest_requests_total",
			Help: "Total number of document ingestion requests.",
		},
		[]string{"mode"},
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
	mux.Handle("/documents", app.instrumentHandler("/documents", http.HandlerFunc(app.handleDocuments)))
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
		"endpoints":   "/health, /ready, /collections, /collections/init, /documents, /metrics",
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

func (a *App) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error: "method not allowed",
		})
		return
	}

	var req DocumentIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid json body",
		})
		return
	}

	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.Title = strings.TrimSpace(req.Title)
	req.Source = strings.TrimSpace(req.Source)
	req.Content = strings.TrimSpace(req.Content)

	if req.DocumentID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "document_id is required",
		})
		return
	}

	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "title is required",
		})
		return
	}

	if req.Source == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "source is required",
		})
		return
	}

	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "content is required",
		})
		return
	}

	if req.ChunkSize == 0 {
		req.ChunkSize = 800
	}

	if req.ChunkOverlap == 0 {
		req.ChunkOverlap = 120
	}

	if req.ChunkSize < 200 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "chunk_size must be at least 200",
		})
		return
	}

	if req.ChunkOverlap < 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "chunk_overlap must be 0 or greater",
		})
		return
	}

	if req.ChunkOverlap >= req.ChunkSize {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "chunk_overlap must be smaller than chunk_size",
		})
		return
	}

	chunkTexts := chunkText(req.Content, req.ChunkSize, req.ChunkOverlap)
	chunks := make([]DocumentChunk, 0, len(chunkTexts))

	for idx, text := range chunkTexts {
		chunkID := fmt.Sprintf("%s::chunk-%03d", req.DocumentID, idx)

		chunk := DocumentChunk{
			ChunkID:    chunkID,
			ChunkIndex: idx,
			Text:       text,
			Metadata: map[string]interface{}{
				"document_id":      req.DocumentID,
				"title":            req.Title,
				"source":           req.Source,
				"chunk_index":      idx,
				"chunk_count":      len(chunkTexts),
				"chunking_version": chunkingVersion,
			},
		}

		chunks = append(chunks, chunk)
	}

	mode := "dry_run"
	if !req.DryRun {
		mode = "planned_ingest"
	}
	documentIngestRequestsTotal.WithLabelValues(mode).Inc()
	documentChunksProducedTotal.WithLabelValues(req.DocumentID).Add(float64(len(chunks)))

	if !req.DryRun {
		writeJSON(w, http.StatusNotImplemented, map[string]interface{}{
			"status":  "not_implemented",
			"message": "real embeddings + qdrant upsert will be enabled after embeddings backend is connected",
			"preview": DocumentIngestResponse{
				Status:       "preview_ready",
				DocumentID:   req.DocumentID,
				Title:        req.Title,
				Source:       req.Source,
				ChunkSize:    req.ChunkSize,
				ChunkOverlap: req.ChunkOverlap,
				ChunkCount:   len(chunks),
				ChunkingMode: "character_window",
				ChunkingVer:  chunkingVersion,
				DryRun:       true,
				Chunks:       chunks,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, DocumentIngestResponse{
		Status:       "preview_ready",
		DocumentID:   req.DocumentID,
		Title:        req.Title,
		Source:       req.Source,
		ChunkSize:    req.ChunkSize,
		ChunkOverlap: req.ChunkOverlap,
		ChunkCount:   len(chunks),
		ChunkingMode: "character_window",
		ChunkingVer:  chunkingVersion,
		DryRun:       true,
		Chunks:       chunks,
	})
}

func chunkText(content string, chunkSize int, chunkOverlap int) []string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) == 0 {
		return []string{}
	}

	step := chunkSize - chunkOverlap
	if step <= 0 {
		step = chunkSize
	}

	var chunks []string

	for start := 0; start < len(runes); start += step {
		end := start + chunkSize
		if end > len(runes) {
			end = len(runes)
		}

		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		if end == len(runes) {
			break
		}
	}

	return chunks
}

func (a *App) doQdrantRequest(method, path string, payload interface{}) (*http.Response, []byte, error) {
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

func writeJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
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
