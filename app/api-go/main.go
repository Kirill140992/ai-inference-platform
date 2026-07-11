package main

import (
	"bytes"
	"crypto/sha256"
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
	httpClient     *http.Client
	qdrantURL      string
	vllmURL        string
	embeddingModel string
	embeddingDim   int
	collectionName string
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type ReadyResponse struct {
	Status     string          `json:"status"`
	Service    string          `json:"service"`
	QdrantURL  string          `json:"qdrant_url"`
	Qdrant     json.RawMessage `json:"qdrant,omitempty"`
	VLLMURL    string          `json:"vllm_url,omitempty"`
	VLLMModels json.RawMessage `json:"vllm_models,omitempty"`
	Collection string          `json:"collection,omitempty"`
	Error      string          `json:"error,omitempty"`
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
	Status        string          `json:"status"`
	DocumentID    string          `json:"document_id"`
	Title         string          `json:"title"`
	Source        string          `json:"source"`
	ChunkSize     int             `json:"chunk_size"`
	ChunkOverlap  int             `json:"chunk_overlap"`
	ChunkCount    int             `json:"chunk_count"`
	ChunkingMode  string          `json:"chunking_mode"`
	ChunkingVer   string          `json:"chunking_version"`
	DryRun        bool            `json:"dry_run"`
	UpsertedCount int             `json:"upserted_count,omitempty"`
	Chunks        []DocumentChunk `json:"chunks"`
}

type SearchRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

type SearchResult struct {
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type SearchResponse struct {
	Query      string         `json:"query"`
	Collection string         `json:"collection"`
	TopK       int            `json:"top_k"`
	Results    []SearchResult `json:"results"`
}

// EmbeddingsAPIRequest/Response model the OpenAI-compatible /v1/embeddings
// contract served by vLLM (and by cmd/mock-vllm for local dev).
type EmbeddingsAPIRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbeddingsAPIResponse struct {
	Data []EmbeddingsAPIData `json:"data"`
}

type EmbeddingsAPIData struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type QdrantPoint struct {
	ID      string                 `json:"id"`
	Vector  []float64              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

type QdrantUpsertRequest struct {
	Points []QdrantPoint `json:"points"`
}

type QdrantSearchRequest struct {
	Vector      []float64 `json:"vector"`
	Limit       int       `json:"limit"`
	WithPayload bool      `json:"with_payload"`
}

type QdrantSearchResponse struct {
	Result []QdrantScoredPoint `json:"result"`
}

type QdrantScoredPoint struct {
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type QdrantCollectionInfoResponse struct {
	Result struct {
		Config struct {
			Params struct {
				Vectors QdrantVectorsConfig `json:"vectors"`
			} `json:"params"`
		} `json:"config"`
	} `json:"result"`
}

type QdrantScrollRequest struct {
	Limit       int  `json:"limit"`
	WithPayload bool `json:"with_payload"`
}

type QdrantScrollResponse struct {
	Result struct {
		Points []struct {
			Payload map[string]interface{} `json:"payload"`
		} `json:"points"`
	} `json:"result"`
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

const chunkingVersion = "chunker_v2"

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

	// Metric names are pinned by k8s/monitoring/api-go-dashboard.yaml and
	// docs/book-issues/00-bringup-vast-tailscale.md — do not rename.
	vllmRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "vllm_request_duration_seconds",
			Help:    "Duration of outbound requests from api-go to the vLLM backend.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	vllmRequestErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "vllm_request_errors_total",
			Help: "Total number of outbound request errors from api-go to the vLLM backend.",
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
	// Defaults match cmd/mock-vllm so the local dev loop needs zero config;
	// k8s manifests must set these explicitly for the real backend.
	vllmURL := strings.TrimRight(getEnv("VLLM_URL", "http://localhost:8000/v1"), "/")
	embeddingModel := getEnv("EMBEDDING_MODEL", "mock-embedding-model")
	embeddingDim := getEnvInt("EMBEDDING_DIM", 384)
	collectionName := getEnv("QDRANT_COLLECTION", "knowledge")

	app := &App{
		httpClient: &http.Client{
			// Generous enough for embedding a full document batch on a real
			// GPU backend; the mock answers instantly either way.
			Timeout: 30 * time.Second,
		},
		qdrantURL:      qdrantURL,
		vllmURL:        vllmURL,
		embeddingModel: embeddingModel,
		embeddingDim:   embeddingDim,
		collectionName: collectionName,
	}

	mux := http.NewServeMux()

	mux.Handle("/", app.instrumentHandler("/", http.HandlerFunc(app.handleRoot)))
	mux.Handle("/health", app.instrumentHandler("/health", http.HandlerFunc(app.handleHealth)))
	mux.Handle("/ready", app.instrumentHandler("/ready", http.HandlerFunc(app.handleReady)))
	mux.Handle("/collections", app.instrumentHandler("/collections", http.HandlerFunc(app.handleCollections)))
	mux.Handle("/collections/init", app.instrumentHandler("/collections/init", http.HandlerFunc(app.handleCollectionsInit)))
	mux.Handle("/documents", app.instrumentHandler("/documents", http.HandlerFunc(app.handleDocuments)))
	mux.Handle("/search", app.instrumentHandler("/search", http.HandlerFunc(app.handleSearch)))
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("starting api-go on port %s", port)
	log.Printf("using qdrant url: %s (collection %q)", qdrantURL, collectionName)
	log.Printf("using vllm url: %s (embedding model %q, dim %d)", vllmURL, embeddingModel, embeddingDim)

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
		"endpoints":   "/health, /ready, /collections, /collections/init, /documents, /search, /metrics",
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Service: "api-go",
	})
}

func (a *App) handleReady(w http.ResponseWriter, r *http.Request) {
	notReady := func(errMsg string, qdrantBody, vllmBody json.RawMessage) {
		apiReadinessFailuresTotal.Inc()
		writeJSON(w, http.StatusServiceUnavailable, ReadyResponse{
			Status:     "not ready",
			Service:    "api-go",
			QdrantURL:  a.qdrantURL,
			Qdrant:     qdrantBody,
			VLLMURL:    a.vllmURL,
			VLLMModels: vllmBody,
			Collection: a.collectionName,
			Error:      errMsg,
		})
	}

	resp, qdrantBody, err := a.doQdrantRequest(http.MethodGet, "/", nil)
	if err != nil {
		notReady("qdrant: "+err.Error(), nil, nil)
		return
	}
	if resp.StatusCode >= 400 {
		notReady(fmt.Sprintf("qdrant returned status %d", resp.StatusCode), nil, nil)
		return
	}

	// Dependency health: vLLM must answer /models, otherwise ingest and
	// search would fail on the first embedding call anyway.
	vllmResp, vllmBody, err := a.doVLLMRequest(http.MethodGet, "/models", nil)
	if err != nil {
		notReady("vllm: "+err.Error(), qdrantBody, nil)
		return
	}
	if vllmResp.StatusCode >= 400 {
		notReady(fmt.Sprintf("vllm returned status %d", vllmResp.StatusCode), qdrantBody, nil)
		return
	}

	// Hard rule: refuse to look ready when the configured embedding
	// model/dimension contradicts what the collection was created with.
	if _, err := a.checkCollectionCompat(a.collectionName); err != nil {
		notReady("collection: "+err.Error(), qdrantBody, vllmBody)
		return
	}

	writeJSON(w, http.StatusOK, ReadyResponse{
		Status:     "ready",
		Service:    "api-go",
		QdrantURL:  a.qdrantURL,
		Qdrant:     qdrantBody,
		VLLMURL:    a.vllmURL,
		VLLMModels: vllmBody,
		Collection: a.collectionName,
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
		req.Name = a.collectionName
	}

	if req.VectorSize == 0 {
		req.VectorSize = a.embeddingDim
	}

	// Hard rule, enforced not just documented: a collection whose size differs
	// from EMBEDDING_DIM would silently break retrieval on the first query.
	if req.VectorSize != a.embeddingDim {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf(
				"vector_size %d does not match EMBEDDING_DIM %d — collections must be created with the embedding model's dimension",
				req.VectorSize, a.embeddingDim,
			),
		})
		return
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
		mode = "ingest"
	}
	documentIngestRequestsTotal.WithLabelValues(mode).Inc()
	documentChunksProducedTotal.WithLabelValues(req.DocumentID).Add(float64(len(chunks)))

	if !req.DryRun {
		a.ingestChunks(w, req, chunkTexts, chunks)
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
		ChunkingMode: "markdown_aware",
		ChunkingVer:  chunkingVersion,
		DryRun:       true,
		Chunks:       chunks,
	})
}

// ingestChunks is the real (non-dry-run) ingestion path: embed every chunk
// via the vLLM backend, then upsert one point per chunk into Qdrant.
func (a *App) ingestChunks(w http.ResponseWriter, req DocumentIngestRequest, chunkTexts []string, chunks []DocumentChunk) {
	exists, err := a.checkCollectionCompat(a.collectionName)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		return
	}
	if !exists {
		writeJSON(w, http.StatusConflict, ErrorResponse{
			Error: fmt.Sprintf("collection %q does not exist — create it first via POST /collections/init", a.collectionName),
		})
		return
	}

	vectors, err := a.embedTexts(chunkTexts)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "embedding backend: " + err.Error()})
		return
	}

	points := make([]QdrantPoint, len(chunks))
	for i, chunk := range chunks {
		payload := map[string]interface{}{
			"text":            chunk.Text,
			"chunk_id":        chunk.ChunkID,
			"embedding_model": a.embeddingModel,
			"embedding_dim":   a.embeddingDim,
		}
		for k, v := range chunk.Metadata {
			payload[k] = v
		}

		points[i] = QdrantPoint{
			ID:      pointIDFromChunkID(chunk.ChunkID),
			Vector:  vectors[i],
			Payload: payload,
		}
	}

	upsertResp, upsertBody, err := a.doQdrantRequest(
		http.MethodPut,
		"/collections/"+url.PathEscape(a.collectionName)+"/points?wait=true",
		QdrantUpsertRequest{Points: points},
	)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "qdrant upsert: " + err.Error()})
		return
	}
	if upsertResp.StatusCode >= 400 {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{
			Error: fmt.Sprintf("qdrant upsert returned status %d: %s", upsertResp.StatusCode, truncateBody(upsertBody)),
		})
		return
	}

	writeJSON(w, http.StatusOK, DocumentIngestResponse{
		Status:        "ingested",
		DocumentID:    req.DocumentID,
		Title:         req.Title,
		Source:        req.Source,
		ChunkSize:     req.ChunkSize,
		ChunkOverlap:  req.ChunkOverlap,
		ChunkCount:    len(chunks),
		ChunkingMode:  "markdown_aware",
		ChunkingVer:   chunkingVersion,
		DryRun:        false,
		UpsertedCount: len(points),
		Chunks:        chunks,
	})
}

// handleSearch is the retrieval path: embed the query with the same model the
// collection was ingested with, then return top-k nearest chunks from Qdrant.
func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error: "method not allowed",
		})
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid json body",
		})
		return
	}

	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "query is required",
		})
		return
	}

	if req.TopK == 0 {
		req.TopK = 5
	}
	if req.TopK < 1 || req.TopK > 50 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "top_k must be between 1 and 50",
		})
		return
	}

	exists, err := a.checkCollectionCompat(a.collectionName)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
		return
	}
	if !exists {
		writeJSON(w, http.StatusConflict, ErrorResponse{
			Error: fmt.Sprintf("collection %q does not exist — create it first via POST /collections/init", a.collectionName),
		})
		return
	}

	vectors, err := a.embedTexts([]string{req.Query})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "embedding backend: " + err.Error()})
		return
	}

	searchResp, searchBody, err := a.doQdrantRequest(
		http.MethodPost,
		"/collections/"+url.PathEscape(a.collectionName)+"/points/search",
		QdrantSearchRequest{Vector: vectors[0], Limit: req.TopK, WithPayload: true},
	)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "qdrant search: " + err.Error()})
		return
	}
	if searchResp.StatusCode >= 400 {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{
			Error: fmt.Sprintf("qdrant search returned status %d: %s", searchResp.StatusCode, truncateBody(searchBody)),
		})
		return
	}

	var parsed QdrantSearchResponse
	if err := json.Unmarshal(searchBody, &parsed); err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{
			Error: "failed to parse qdrant search response",
		})
		return
	}

	results := make([]SearchResult, 0, len(parsed.Result))
	for _, hit := range parsed.Result {
		results = append(results, SearchResult{Score: hit.Score, Payload: hit.Payload})
	}

	writeJSON(w, http.StatusOK, SearchResponse{
		Query:      req.Query,
		Collection: a.collectionName,
		TopK:       req.TopK,
		Results:    results,
	})
}

func chunkText(content string, chunkSize int, chunkOverlap int) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSpace(content)

	if content == "" {
		return []string{}
	}

	sections := splitMarkdownSections(content)
	var chunks []string

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		if len([]rune(section)) <= chunkSize {
			chunks = append(chunks, section)
			continue
		}

		paragraphChunks := chunkSectionByParagraphs(section, chunkSize, chunkOverlap)
		chunks = append(chunks, paragraphChunks...)
	}

	return chunks
}

func splitMarkdownSections(content string) []string {
	lines := strings.Split(content, "\n")

	var sections []string
	var current []string

	flush := func() {
		section := strings.TrimSpace(strings.Join(current, "\n"))
		if section != "" {
			sections = append(sections, section)
		}
		current = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isHeading := strings.HasPrefix(trimmed, "#")

		if isHeading && len(current) > 0 {
			flush()
		}

		current = append(current, line)
	}

	if len(current) > 0 {
		flush()
	}

	return sections
}

func chunkSectionByParagraphs(section string, chunkSize int, chunkOverlap int) []string {
	paragraphs := splitParagraphs(section)

	var chunks []string
	var current string

	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		if len([]rune(paragraph)) > chunkSize {
			if strings.TrimSpace(current) != "" {
				chunks = append(chunks, strings.TrimSpace(current))
				current = ""
			}

			longChunks := splitLongTextByWords(paragraph, chunkSize, chunkOverlap)
			chunks = append(chunks, longChunks...)
			continue
		}

		if strings.TrimSpace(current) == "" {
			current = paragraph
			continue
		}

		candidate := current + "\n\n" + paragraph
		if len([]rune(candidate)) <= chunkSize {
			current = candidate
		} else {
			chunks = append(chunks, strings.TrimSpace(current))
			current = paragraph
		}
	}

	if strings.TrimSpace(current) != "" {
		chunks = append(chunks, strings.TrimSpace(current))
	}

	return chunks
}

func splitParagraphs(section string) []string {
	lines := strings.Split(section, "\n")

	var paragraphs []string
	var current []string

	flush := func() {
		paragraph := strings.TrimSpace(strings.Join(current, "\n"))
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
		current = nil
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				flush()
			}
			continue
		}

		current = append(current, line)
	}

	if len(current) > 0 {
		flush()
	}

	return paragraphs
}

func splitLongTextByWords(text string, chunkSize int, chunkOverlap int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var chunks []string
	start := 0

	for start < len(words) {
		currentWords := []string{}
		currentLen := 0
		end := start

		for end < len(words) {
			wordLen := len([]rune(words[end]))
			addLen := wordLen
			if len(currentWords) > 0 {
				addLen += 1
			}

			if currentLen+addLen > chunkSize && len(currentWords) > 0 {
				break
			}

			currentWords = append(currentWords, words[end])
			currentLen += addLen
			end++

			if currentLen >= chunkSize {
				break
			}
		}

		chunk := strings.TrimSpace(strings.Join(currentWords, " "))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}

		if end >= len(words) {
			break
		}

		overlapWords := calculateOverlapWords(currentWords, chunkOverlap)

		if overlapWords >= len(currentWords) {
			overlapWords = len(currentWords) - 1
		}
		if overlapWords < 0 {
			overlapWords = 0
		}

		start = end - overlapWords
		if start >= end {
			start = end
		}
	}

	return chunks
}

func calculateOverlapWords(words []string, overlapChars int) int {
	if overlapChars <= 0 || len(words) <= 1 {
		return 0
	}

	totalChars := 0
	count := 0

	for i := len(words) - 1; i >= 0; i-- {
		wordLen := len([]rune(words[i]))
		totalChars += wordLen
		if count > 0 {
			totalChars += 1
		}
		count++

		if totalChars >= overlapChars {
			break
		}
	}

	if count >= len(words) {
		count = len(words) - 1
	}
	if count < 0 {
		count = 0
	}

	return count
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

// doVLLMRequest mirrors doQdrantRequest for the vLLM backend, feeding the
// vllm_* metrics that k8s/monitoring/api-go-dashboard.yaml visualizes.
func (a *App) doVLLMRequest(method, path string, payload interface{}) (*http.Response, []byte, error) {
	startedAt := time.Now()

	var bodyReader io.Reader

	if payload != nil {
		requestBody, err := json.Marshal(payload)
		if err != nil {
			vllmRequestErrorsTotal.WithLabelValues(method, path).Inc()
			return nil, nil, fmt.Errorf("failed to marshal vllm request: %w", err)
		}
		bodyReader = bytes.NewReader(requestBody)
	}

	req, err := http.NewRequest(method, a.vllmURL+path, bodyReader)
	if err != nil {
		vllmRequestErrorsTotal.WithLabelValues(method, path).Inc()
		return nil, nil, fmt.Errorf("failed to create request to vllm: %w", err)
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		vllmRequestErrorsTotal.WithLabelValues(method, path).Inc()
		return nil, nil, fmt.Errorf("failed to reach vllm: %w", err)
	}

	body, err := readResponseBody(resp)
	if err != nil {
		vllmRequestErrorsTotal.WithLabelValues(method, path).Inc()
		return resp, nil, fmt.Errorf("failed to read vllm response: %w", err)
	}

	vllmRequestDuration.WithLabelValues(
		method,
		path,
		strconv.Itoa(resp.StatusCode),
	).Observe(time.Since(startedAt).Seconds())

	return resp, body, nil
}

// embedTexts calls the OpenAI-compatible /embeddings endpoint and returns one
// vector per input text, in input order. Count and dimension are validated so
// a misconfigured backend fails loudly instead of corrupting the collection.
func (a *App) embedTexts(texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return [][]float64{}, nil
	}

	resp, body, err := a.doVLLMRequest(http.MethodPost, "/embeddings", EmbeddingsAPIRequest{
		Model: a.embeddingModel,
		Input: texts,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("vllm embeddings returned status %d: %s", resp.StatusCode, truncateBody(body))
	}

	var parsed EmbeddingsAPIResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse vllm embeddings response: %w", err)
	}

	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("vllm returned %d embeddings for %d inputs", len(parsed.Data), len(texts))
	}

	vectors := make([][]float64, len(texts))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(texts) {
			return nil, fmt.Errorf("vllm returned embedding with out-of-range index %d", item.Index)
		}
		if len(item.Embedding) != a.embeddingDim {
			return nil, fmt.Errorf(
				"embedding dimension mismatch: vllm returned %d, EMBEDDING_DIM is %d — model and collection config must agree",
				len(item.Embedding), a.embeddingDim,
			)
		}
		vectors[item.Index] = item.Embedding
	}

	for i, vec := range vectors {
		if vec == nil {
			return nil, fmt.Errorf("vllm response is missing an embedding for input %d", i)
		}
	}

	return vectors, nil
}

// checkCollectionCompat enforces the embedding hard rule: the collection's
// vector size must match EMBEDDING_DIM, and already-ingested points must have
// been embedded with the same EMBEDDING_MODEL (checked via the payload stamp
// written on ingest). Returns exists=false when the collection is absent —
// that's not an error, /collections/init creates it with the right size.
func (a *App) checkCollectionCompat(collection string) (bool, error) {
	resp, body, err := a.doQdrantRequest(http.MethodGet, "/collections/"+url.PathEscape(collection), nil)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Errorf("qdrant returned status %d for collection %q", resp.StatusCode, collection)
	}

	var info QdrantCollectionInfoResponse
	if err := json.Unmarshal(body, &info); err != nil {
		return true, fmt.Errorf("failed to parse collection info for %q: %w", collection, err)
	}

	if size := info.Result.Config.Params.Vectors.Size; size != 0 && size != a.embeddingDim {
		return true, fmt.Errorf(
			"embedding dimension mismatch: collection %q was created with size %d but EMBEDDING_DIM is %d — refusing to mix incompatible vectors",
			collection, size, a.embeddingDim,
		)
	}

	scrollResp, scrollBody, err := a.doQdrantRequest(
		http.MethodPost,
		"/collections/"+url.PathEscape(collection)+"/points/scroll",
		QdrantScrollRequest{Limit: 1, WithPayload: true},
	)
	if err != nil {
		return true, err
	}
	if scrollResp.StatusCode >= 400 {
		return true, fmt.Errorf("qdrant scroll returned status %d for collection %q", scrollResp.StatusCode, collection)
	}

	var scroll QdrantScrollResponse
	if err := json.Unmarshal(scrollBody, &scroll); err != nil {
		return true, fmt.Errorf("failed to parse scroll response for %q: %w", collection, err)
	}

	for _, point := range scroll.Result.Points {
		storedModel, ok := point.Payload["embedding_model"].(string)
		if ok && storedModel != a.embeddingModel {
			return true, fmt.Errorf(
				"embedding model mismatch: collection %q holds vectors from %q but EMBEDDING_MODEL is %q — the same text would embed to incompatible vectors",
				collection, storedModel, a.embeddingModel,
			)
		}
	}

	return true, nil
}

// pointIDFromChunkID derives a stable UUID for a chunk from its chunk ID.
// Qdrant only accepts UUIDs or unsigned ints as point IDs; hashing the chunk
// ID keeps re-ingestion idempotent (same chunk → same point, upsert replaces
// it instead of duplicating). The original chunk_id stays in the payload.
func pointIDFromChunkID(chunkID string) string {
	sum := sha256.Sum256([]byte(chunkID))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x40 // version bits: UUIDv4 layout
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func truncateBody(body []byte) string {
	const max = 200
	s := string(body)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
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

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("invalid %s=%q, using default %d", key, value, fallback)
		return fallback
	}
	return parsed
}
