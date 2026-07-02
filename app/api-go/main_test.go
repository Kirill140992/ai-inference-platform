package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

// TestHandleDocumentsNonDryRunStillStubbed pins today's #0-pending behavior:
// a real (non-dry-run) ingest request returns 501 because embeddings/Qdrant
// upsert aren't wired yet. Once #0 lands this test should start failing —
// that's the signal to replace it with a test of the real ingest path.
func TestHandleDocumentsNonDryRunStillStubbed(t *testing.T) {
	app := &App{httpClient: &http.Client{}, qdrantURL: "http://unused.invalid"}

	body := `{"document_id":"d1","title":"t","source":"s","content":"# Heading\n\nSome content.","dry_run":false}`
	req := httptest.NewRequest(http.MethodPost, "/documents", strings.NewReader(body))
	rec := httptest.NewRecorder()

	app.handleDocuments(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}
