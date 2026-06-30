## Goal
Add a reranking stage after Qdrant vector search to raise top-k precision.

## Why now
- Closes a documented gap: `demo-docs/knowledge-ingestion-design.md` → "Known limitations #1: No reranking" and `architecture-overview.md` → "no reranker".
- Applied task for **AI Engineering (Chip Huyen) — RAG & Agents chapter**.

## Scope
- `api-go` retrieval: fetch top-N candidates from Qdrant (e.g. N = 20).
- Rerank candidates with a cross-encoder (e.g. `bge-reranker-v2-m3`) served on the vLLM GPU host or a small dedicated endpoint.
- Return top-k (e.g. k = 5) after reranking.
- Expose Prometheus metrics: rerank latency, candidates in/out.
- Make N, k, and the reranker on/off switch configurable.

## Acceptance criteria
- [ ] Reranking toggle + params in config.
- [ ] Rerank latency visible in Grafana.
- [ ] Before/after comparison on a handful of hard queries, written up in an ADR (`demo-docs/adr-00X-reranker.md`).
- [ ] Graceful fallback to raw vector results if the reranker endpoint is down.

## References
- AI Engineering — RAG & Agents (retrieval quality / reranking).
- Measured by the offline-eval issue; pairs with hybrid-search.
