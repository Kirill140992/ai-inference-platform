package main

import (
	"fmt"
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
