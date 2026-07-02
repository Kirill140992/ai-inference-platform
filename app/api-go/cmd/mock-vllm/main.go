package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type modelsResponse struct {
	Object string          `json:"object"`
	Data   []modelListItem `json:"data"`
}

type modelListItem struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type embeddingsRequest struct {
	Model string      `json:"model"`
	Input interface{} `json:"input"`
}

type embeddingsResponse struct {
	Object string      `json:"object"`
	Data   []embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  usage       `json:"usage"`
}

type embedding struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type usage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func main() {
	port := flag.String("port", getEnv("PORT", "8000"), "port to listen on")
	dim := flag.Int("dim", getEnvInt("EMBEDDING_DIM", 384), "embedding vector dimension")
	model := flag.String("model", getEnv("EMBEDDING_MODEL", "mock-embedding-model"), "model name reported by /v1/models and /v1/embeddings")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", handleModels(*model))
	mux.HandleFunc("/v1/embeddings", handleEmbeddings(*model, *dim))

	server := &http.Server{
		Addr:              ":" + *port,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("mock-vllm: serving OpenAI-compatible /v1/models + /v1/embeddings on :%s (model=%q, dim=%d)", *port, *model, *dim)
	log.Printf("mock-vllm: point api-go's VLLM_URL at http://<this-host>:%s/v1 for local dev — no Vast.ai/Tailscale needed", *port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("mock-vllm: server failed: %v", err)
	}
}

func handleModels(model string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, modelsResponse{
			Object: "list",
			Data: []modelListItem{
				{ID: model, Object: "model", Created: 0, OwnedBy: "mock-vllm"},
			},
		})
	}
}

func handleEmbeddings(model string, dim int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req embeddingsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}

		inputs, err := normalizeInput(req.Input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if len(inputs) == 0 {
			writeError(w, http.StatusBadRequest, "input must not be empty")
			return
		}

		data := make([]embedding, len(inputs))
		totalTokens := 0
		for i, text := range inputs {
			data[i] = embedding{
				Object:    "embedding",
				Index:     i,
				Embedding: fakeEmbedding(text, dim),
			}
			totalTokens += approxTokenCount(text)
		}

		writeJSON(w, http.StatusOK, embeddingsResponse{
			Object: "list",
			Data:   data,
			Model:  model,
			Usage:  usage{PromptTokens: totalTokens, TotalTokens: totalTokens},
		})
	}
}

func normalizeInput(raw interface{}) ([]string, error) {
	switch v := raw.(type) {
	case string:
		return []string{v}, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("input array must contain only strings")
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("input must be a string or an array of strings")
	}
}

// fakeEmbedding deterministically derives a unit-length pseudo-random vector
// from the input text: the same chunk always embeds to the same vector
// across calls and restarts, which is what retrieval-logic tests need, even
// though the vector carries no real semantic meaning.
func fakeEmbedding(text string, dim int) []float64 {
	seed := sha256.Sum256([]byte(text))

	vec := make([]float64, dim)
	var norm float64
	for i := 0; i < dim; i++ {
		idx := []byte{byte(i), byte(i >> 8), byte(i >> 16), byte(i >> 24)}
		h := sha256.Sum256(append(seed[:], idx...))
		u := binary.BigEndian.Uint64(h[:8])
		v := (float64(u)/float64(math.MaxUint64))*2 - 1
		vec[i] = v
		norm += v * v
	}

	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec
}

func approxTokenCount(text string) int {
	return len(strings.Fields(text))
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: errorDetail{Message: message, Type: "invalid_request_error"}})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
