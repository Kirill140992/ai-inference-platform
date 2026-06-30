## Goal
Benchmark and tune the vLLM endpoint (embeddings + generation) for latency, throughput, and cost.

## Why now
- Direct DevOps → MLOps bridge; applied task for **AI Engineering (Chip Huyen) — Inference Optimization chapter**.
- Targets the most failure-prone path in your own docs: `api-go -> external GPU host -> vLLM`.

## Scope
- Repeatable load test against the OpenAI-compatible vLLM endpoint (`k6` / `vegeta` / `locust`, or vLLM's bench scripts).
- Measure p50/p95 latency, tokens/s, and max QPS across concurrency and batch-size settings.
- Test at least one optimization lever (continuous-batching params and/or quantization).
- Export vLLM latency/throughput to Prometheus; add a Grafana panel.

## Acceptance criteria
- [ ] Benchmark script in `scripts/bench/`.
- [ ] Before/after results table (baseline vs one tuning lever).
- [ ] Grafana panel for vLLM latency/throughput.
- [ ] ADR with findings + recommended settings.

## References
- AI Engineering — Inference Optimization.
- Connects to ROADMAP item "Distributed Tracing (OTel / Jaeger)".
