package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestChunkTextEmptyContent(t *testing.T) {
	got := chunkText("", 800, 120)
	if len(got) != 0 {
		t.Fatalf("expected no chunks for empty content, got %d: %#v", len(got), got)
	}
}

func TestChunkTextShortContentSingleChunk(t *testing.T) {
	content := "# Title\n\nA short paragraph that fits in one chunk."

	got := chunkText(content, 800, 120)

	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d: %#v", len(got), got)
	}
	if got[0] != content {
		t.Fatalf("expected chunk to equal the trimmed input, got %q", got[0])
	}
}

// TestChunkTextLongParagraphSplitsWithOverlap checks properties of the
// splitter rather than exact chunk boundaries (which are an implementation
// detail): every source word must survive somewhere, and consecutive chunks
// must actually overlap when chunkOverlap > 0 — that's the invariant callers
// (embedding continuity across chunk boundaries) depend on.
func TestChunkTextLongParagraphSplitsWithOverlap(t *testing.T) {
	words := make([]string, 20)
	for i := range words {
		words[i] = fmt.Sprintf("word%02d", i)
	}
	content := strings.Join(words, " ")

	got := chunkText(content, 20, 6)

	if len(got) < 2 {
		t.Fatalf("expected content longer than chunkSize to split into multiple chunks, got %d: %#v", len(got), got)
	}

	seen := make(map[string]bool)
	for _, chunk := range got {
		if strings.TrimSpace(chunk) == "" {
			t.Fatalf("chunkText produced an empty chunk: %#v", got)
		}
		for _, w := range strings.Fields(chunk) {
			seen[w] = true
		}
	}
	for _, w := range words {
		if !seen[w] {
			t.Fatalf("word %q from the source content is missing from every chunk: %#v", w, got)
		}
	}

	for i := 0; i < len(got)-1; i++ {
		curWords := strings.Fields(got[i])
		nextWords := strings.Fields(got[i+1])
		lastOfCur := curWords[len(curWords)-1]

		overlaps := false
		for _, w := range nextWords {
			if w == lastOfCur {
				overlaps = true
				break
			}
		}
		if !overlaps {
			t.Fatalf("expected chunk %d and chunk %d to overlap by at least one word, got %v | %v", i, i+1, curWords, nextWords)
		}
	}
}

func TestNormalizeDistance(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty defaults to cosine", "", "Cosine", false},
		{"lowercase cosine", "cosine", "Cosine", false},
		{"mixed case", "CoSiNe", "Cosine", false},
		{"dot", "dot", "Dot", false},
		{"euclid", "euclid", "Euclid", false},
		{"manhattan", "manhattan", "Manhattan", false},
		{"unsupported value errors", "jaccard", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeDistance(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeDistance(%q): expected an error, got none", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeDistance(%q): unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeDistance(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestHandleDocumentsValidation(t *testing.T) {
	app := &App{httpClient: &http.Client{}, qdrantURL: "http://unused.invalid"}

	cases := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{
			name:       "wrong method rejected",
			method:     http.MethodGet,
			body:       "",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "invalid json body rejected",
			method:     http.MethodPost,
			body:       "{not json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing document_id rejected",
			method:     http.MethodPost,
			body:       `{"title":"t","source":"s","content":"c","dry_run":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "chunk_size below minimum rejected",
			method:     http.MethodPost,
			body:       `{"document_id":"d1","title":"t","source":"s","content":"some content long enough","chunk_size":100,"dry_run":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "chunk_overlap not smaller than chunk_size rejected",
			method:     http.MethodPost,
			body:       `{"document_id":"d1","title":"t","source":"s","content":"some content long enough","chunk_size":300,"chunk_overlap":300,"dry_run":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid dry run request succeeds",
			method:     http.MethodPost,
			body:       `{"document_id":"d1","title":"t","source":"s","content":"# Heading\n\nSome content.","dry_run":true}`,
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/documents", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			app.handleDocuments(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestHandleDocumentsRealIngestUpserts wires handleDocuments against stub
// vLLM and Qdrant servers (#0 Checkpoint 2): a non-dry-run request must embed
// every chunk and upsert exactly one point per chunk, stamped with the
// embedding model and carrying a vector of the configured dimension.
func TestHandleDocumentsRealIngestUpserts(t *testing.T) {
	const dim = 8

	vllm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected vllm request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}

		var req EmbeddingsAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("bad embeddings request body: %v", err)
		}

		data := make([]map[string]interface{}, len(req.Input))
		for i := range req.Input {
			vec := make([]float64, dim)
			vec[0] = float64(i + 1)
			data[i] = map[string]interface{}{"index": i, "embedding": vec}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
	}))
	defer vllm.Close()

	var upserted QdrantUpsertRequest
	qdrant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/knowledge":
			_, _ = w.Write([]byte(`{"result":{"config":{"params":{"vectors":{"size":8,"distance":"Cosine"}}}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/knowledge/points/scroll":
			_, _ = w.Write([]byte(`{"result":{"points":[]}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/knowledge/points":
			if err := json.NewDecoder(r.Body).Decode(&upserted); err != nil {
				t.Errorf("bad upsert request body: %v", err)
			}
			_, _ = w.Write([]byte(`{"result":{"status":"acknowledged"},"status":"ok"}`))
		default:
			t.Errorf("unexpected qdrant request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer qdrant.Close()

	app := &App{
		httpClient:     &http.Client{},
		qdrantURL:      qdrant.URL,
		vllmURL:        vllm.URL + "/v1",
		embeddingModel: "test-model",
		embeddingDim:   dim,
		collectionName: "knowledge",
	}

	body := `{"document_id":"d1","title":"t","source":"s","content":"# Heading\n\nSome content.","dry_run":false}`
	req := httptest.NewRequest(http.MethodPost, "/documents", strings.NewReader(body))
	rec := httptest.NewRecorder()

	app.handleDocuments(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(upserted.Points) == 0 {
		t.Fatalf("no points reached the qdrant upsert endpoint")
	}
	for _, p := range upserted.Points {
		if len(p.Vector) != dim {
			t.Fatalf("upserted vector dimension = %d, want %d", len(p.Vector), dim)
		}
		if p.Payload["embedding_model"] != "test-model" {
			t.Fatalf("payload embedding_model = %v, want %q", p.Payload["embedding_model"], "test-model")
		}
		if p.Payload["text"] == "" || p.Payload["text"] == nil {
			t.Fatalf("upserted point is missing the chunk text in its payload")
		}
	}
}

// TestEmbedTextsDimensionMismatchFailsLoudly pins the hard rule: a backend
// answering with the wrong vector size must be an error, never an upsert.
func TestEmbedTextsDimensionMismatchFailsLoudly(t *testing.T) {
	vllm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.1,0.2,0.3]}]}`))
	}))
	defer vllm.Close()

	app := &App{httpClient: &http.Client{}, vllmURL: vllm.URL, embeddingModel: "m", embeddingDim: 8}

	_, err := app.embedTexts([]string{"hello"})
	if err == nil {
		t.Fatalf("expected a dimension-mismatch error, got none")
	}
	if !strings.Contains(err.Error(), "dimension mismatch") {
		t.Fatalf("expected a dimension-mismatch error, got: %v", err)
	}
}

func TestHandleCollectionsInitRejectsMismatchedVectorSize(t *testing.T) {
	app := &App{httpClient: &http.Client{}, qdrantURL: "http://unused.invalid", embeddingDim: 384, collectionName: "knowledge"}

	body := `{"name":"knowledge","vector_size":768}`
	req := httptest.NewRequest(http.MethodPost, "/collections/init", strings.NewReader(body))
	rec := httptest.NewRecorder()

	app.handleCollectionsInit(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestPointIDFromChunkIDDeterministicUUID(t *testing.T) {
	a := pointIDFromChunkID("doc::chunk-000")
	b := pointIDFromChunkID("doc::chunk-000")
	c := pointIDFromChunkID("doc::chunk-001")

	if a != b {
		t.Fatalf("same chunk ID produced different point IDs: %q vs %q", a, b)
	}
	if a == c {
		t.Fatalf("different chunk IDs produced the same point ID: %q", a)
	}

	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRe.MatchString(a) {
		t.Fatalf("point ID %q is not a valid RFC-4122 UUID", a)
	}
}
