package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFakeEmbeddingIsDeterministicAndUnitLength(t *testing.T) {
	a := fakeEmbedding("hello world", 16)
	b := fakeEmbedding("hello world", 16)
	c := fakeEmbedding("something else", 16)

	if len(a) != 16 {
		t.Fatalf("expected 16 dimensions, got %d", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("expected identical input to produce identical embeddings, differed at index %d: %v != %v", i, a[i], b[i])
		}
	}

	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatalf("expected different input to produce a different embedding, got identical vectors")
	}

	var norm float64
	for _, v := range a {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 1e-9 {
		t.Fatalf("expected a unit-length vector, got norm %v", norm)
	}
}

func TestNormalizeInput(t *testing.T) {
	if got, err := normalizeInput("solo string"); err != nil || len(got) != 1 || got[0] != "solo string" {
		t.Fatalf("string input: got %v, err %v", got, err)
	}

	if got, err := normalizeInput([]interface{}{"a", "b"}); err != nil || len(got) != 2 {
		t.Fatalf("array input: got %v, err %v", got, err)
	}

	if _, err := normalizeInput([]interface{}{"a", 5}); err == nil {
		t.Fatalf("expected an error for a non-string array element, got none")
	}

	if _, err := normalizeInput(42); err == nil {
		t.Fatalf("expected an error for an unsupported input type, got none")
	}
}

func TestHandleEmbeddingsReturnsExpectedShape(t *testing.T) {
	handler := handleEmbeddings("mock-embedding-model", 8)

	body := `{"model":"mock-embedding-model","input":["chunk one","chunk two"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp embeddingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Model != "mock-embedding-model" {
		t.Fatalf("model = %q, want %q", resp.Model, "mock-embedding-model")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 embeddings for 2 inputs, got %d", len(resp.Data))
	}
	for _, e := range resp.Data {
		if len(e.Embedding) != 8 {
			t.Fatalf("expected embedding dimension 8, got %d", len(e.Embedding))
		}
	}
}

func TestHandleModelsReturnsConfiguredModel(t *testing.T) {
	handler := handleModels("my-model")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	var resp modelsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "my-model" {
		t.Fatalf("expected a single model %q, got %#v", "my-model", resp.Data)
	}
}
