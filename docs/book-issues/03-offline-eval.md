## Goal
Build a fixed offline evaluation set + harness to measure retrieval quality over time.

## Why now
- Closes a documented gap: `demo-docs/knowledge-ingestion-design.md` → "Quality improvement roadmap #4: Offline evaluation set".
- Applied task for **AI Engineering (Chip Huyen) — Evaluation chapters**. Highest-leverage MLOps skill: you can't improve what you don't measure.

## Scope
- Curate 30–50 `query → expected chunk/doc` pairs over the existing `demo-docs` corpus.
  - **Done:** 52 cross-checked pairs live in `eval/retrieval/retrieval-eval.jsonl` (format + curation process in `eval/retrieval/README.md`, verbatim-snippet check in `validate.py`).
- **Harness skeleton in place** (`harness.py`: CLI, dataset loading, lexical-baseline + api retrievers, aggregation, baseline saving). The remaining work — and the point of this issue — is implementing `metrics.py` (recall@k / MRR / nDCG) until `test_metrics.py` passes, then recording the lexical baseline. The api retriever documents the `POST /search` contract #0 Checkpoint 2 should implement.
- Harness computing recall@k, MRR, and nDCG@k.
- One command to run; prints a metrics table; stores a baseline.
- (Optional) wire into CI to flag retrieval regressions on PRs.

## Acceptance criteria
- [ ] Eval dataset committed (`eval/retrieval/*.jsonl`).
- [ ] `make eval` (or a script) outputs recall@k / MRR / nDCG.
- [ ] Baseline numbers recorded in README or an ADR.
- [ ] Used to score the reranker and hybrid-search issues (before/after).

## References
- AI Engineering — Evaluation Methodology / Evaluate AI Systems.
- Gates meaningful acceptance of reranker and hybrid-search.
