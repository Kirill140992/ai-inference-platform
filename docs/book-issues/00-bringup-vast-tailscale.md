## Goal
Stand up the GPU half of the platform so the inference path is real end-to-end: local k3s (api-go + Qdrant) calling **vLLM on a rented Vast.ai GPU** over a private **Tailscale** tunnel, with the **embeddings backend wired** in `api-go`. Unblocks #4, #3, #1, #2.

## Why now
- The vLLM/embeddings backend is currently a **stub** — `app/api-go/main.go` (~L453): *"real embeddings + qdrant upsert will be enabled after embeddings backend is connected"*. `QDRANT_URL` is wired; there is **no** vLLM endpoint yet.
- #4 (benchmark) and #3 (eval) cannot produce real numbers without a live endpoint.
- Implements ROADMAP item #3 (ZTNA to Vast.ai over Tailscale); see `demo-docs/adr-002-why-external-gpu-host.md`.

## Topology (local-first)
`client → api-go (local k3s) → Tailscale → vLLM (Vast.ai GPU) → back → Qdrant (local k3s)`

## Tasks

### A. Vast.ai + vLLM
- [ ] Rent a Vast.ai GPU sized for the chosen embedding model (+ optional small generation model). Note hourly cost.
- [ ] Run vLLM's OpenAI-compatible server with the embedding model. Bind to the **Tailscale interface**, not `0.0.0.0`/public.
- [ ] Record the embedding model name + **vector dimension** (must match the Qdrant collection).

### B. Tailscale
- [ ] Install Tailscale on the Vast host and the local k3s node; join the same tailnet (`tailscale up`).
- [ ] Verify private connectivity (`tailscale ping`, `curl …/v1/models`).
- [ ] ACL: only api-go's node may reach the vLLM port. Auth key handled out-of-band, **never committed**.

### C. Wire api-go (replace the stub)
- [ ] Add env via the existing `getEnv` helper: `VLLM_URL` (e.g. `http://vast-gpu:8000/v1`), `EMBEDDING_MODEL`, `EMBEDDING_DIM`.
- [ ] Implement a real call to `POST {VLLM_URL}/embeddings` where the stub is (~L453).
- [ ] Ingestion: chunk → embed via vLLM → upsert to Qdrant. Query: embed query → Qdrant search.
- [ ] Create the Qdrant collection with `EMBEDDING_DIM` (via the existing `/collections/init`).
- [ ] Add Prometheus metrics for vLLM calls (latency, errors) — mirror the existing qdrant metrics; this pre-builds what #4 measures.
- [ ] `/ready` fails if vLLM is unreachable (dependency health).

### D. Secrets/config
- [ ] vLLM URL / any API key via a k8s Secret (SOPS), not hardcoded. Tailscale auth key out-of-band.

## Acceptance criteria (definition of done)
- [ ] Ingest a few `demo-docs`; vectors land in Qdrant (verify count).
- [ ] A query embeds via vLLM over Tailscale and returns top-k from Qdrant, end-to-end.
- [ ] vLLM latency/error metrics visible in Grafana.
- [ ] ADR `demo-docs/adr-003-vast-tailscale-bringup.md`: topology, model + dimension, cost, ACLs, failure modes.

## Cost / pace note
Vast.ai is hourly — bring the GPU up only during work blocks. For wiring/dev, use a tiny local/CPU embedding model; point at Vast only for real benchmarks (#4).

## References
- `app/api-go/main.go` (~L453 stub, `getEnv`, `QDRANT_URL`, `/collections/init`)
- ROADMAP #3 (ZTNA / Tailscale / Vast.ai) · `demo-docs/adr-002-why-external-gpu-host.md`
- `demo-docs/architecture-overview.md` (failure mode: "vLLM unavailable")
