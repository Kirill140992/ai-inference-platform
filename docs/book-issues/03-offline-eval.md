## Goal
Build a fixed offline evaluation set + harness to measure retrieval quality over time.

## Why now
- Closes a documented gap: `demo-docs/knowledge-ingestion-design.md` → "Quality improvement roadmap #4: Offline evaluation set".
- Applied task for **AI Engineering (Chip Huyen) — Evaluation chapters**. Highest-leverage MLOps skill: you can't improve what you don't measure.

## Scope
- Curate 30–50 `query → expected chunk/doc` pairs over the existing `demo-docs` corpus.
  - **Started:** a 12-pair seed lives in `eval/retrieval/retrieval-eval.jsonl` (format + labeling guidelines in `eval/retrieval/README.md`, verbatim-snippet check in `validate.py`). Labeling doesn't depend on #0 — keep growing it ~5–10 pairs per session in the background.
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
