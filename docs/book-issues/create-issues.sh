#!/usr/bin/env bash
#
# Create 4 applied-learning issues mapped to "AI Engineering" (Chip Huyen).
#
# Prereq: GitHub CLI authenticated as you:
#     gh auth login
# Run from anywhere; the script cd's to its own folder so body-files resolve.
#     bash create-issues.sh
#
set -euo pipefail
cd "$(dirname "$0")"

REPO="Kirill140992/ai-inference-platform"

ensure_label () {
  local name="$1" color="$2" desc="$3"
  gh label create "$name" --repo "$REPO" --color "$color" --description "$desc" 2>/dev/null \
    || gh label edit "$name" --repo "$REPO" --color "$color" --description "$desc" 2>/dev/null \
    || true
}

echo "==> Ensuring labels"
ensure_label "ai-engineering-book" "5319e7" "Applied task from AI Engineering (Chip Huyen)"
ensure_label "rag"                 "0e8a16" "Retrieval-augmented generation"
ensure_label "evaluation"          "fbca04" "Quality evaluation / metrics"
ensure_label "performance"         "d93f0b" "Latency / throughput / cost"

create () {
  local title="$1" labels="$2" body="$3"
  echo "==> Creating: $title"
  gh issue create --repo "$REPO" --title "$title" --label "$labels" --body-file "$body"
}

create "RAG: add a reranking stage to retrieval" \
  "enhancement,rag,ai-engineering-book" "01-reranker.md"
create "RAG: add hybrid (lexical + dense) search" \
  "enhancement,rag,ai-engineering-book" "02-hybrid-search.md"
create "Eval: offline retrieval evaluation set + harness" \
  "enhancement,evaluation,ai-engineering-book" "03-offline-eval.md"
create "Perf: vLLM inference benchmark + optimization" \
  "enhancement,performance,ai-engineering-book" "04-inference-benchmark.md"

echo "==> Done. List them: gh issue list --repo $REPO --label ai-engineering-book"
