## Goal
Combine lexical (sparse / BM25) and dense retrieval, fused with Reciprocal Rank Fusion (RRF).

## Why now
- Closes a documented gap: `demo-docs/knowledge-ingestion-design.md` → "Known limitations #2: No hybrid lexical + semantic search" (weak on IDs, error codes, rare terms).
- Applied task for **AI Engineering (Chip Huyen) — RAG & Agents chapter**.

## Scope
- Add a sparse/lexical index (Qdrant sparse vectors or a BM25 path) alongside the dense collection.
- Run both retrievers per query; fuse results with RRF.
- Make fusion weights and hybrid on/off configurable.

## Acceptance criteria
- [ ] Hybrid path returns fused results end-to-end.
- [ ] Measurable improvement on identifier-heavy queries vs dense-only (scored with the eval set).
- [ ] ADR documenting design + trade-offs.

## References
- AI Engineering — RAG & Agents (hybrid retrieval).
- Pairs with reranker; measured by offline-eval.
