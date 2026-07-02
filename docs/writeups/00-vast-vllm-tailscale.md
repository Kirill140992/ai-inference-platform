# Wiring Real Embeddings: vLLM on a Rented GPU over Tailscale

<!--
WRITEUP TEMPLATE for #0 — fill in after the feature is merged.
Target reader: a hiring manager or senior engineer skimming the repo.
They will spend 3-5 minutes here. Every section has a guidance comment —
delete the comments as you fill the sections in.

Style rules (from CLAUDE.md): English, honest about trade-offs and
failures, numbers over adjectives. If a sentence could appear in any
project's writeup, cut it — keep only what actually happened HERE.
-->

**Status:** <!-- draft / published --> · **Feature:** [#0](../book-issues/00-bringup-vast-tailscale.md) · **ADR:** [ADR-003](../../demo-docs/adr-003-vast-tailscale-bringup.md)

## TL;DR

<!-- 3-4 sentences max, written LAST. What was the gap, what did you
build, what's the one number that proves it works (e.g. end-to-end
ingest→search latency, or "52-pair eval set now runs against live
embeddings"). A reader who stops here should still get the point. -->

## The gap

<!-- 1-2 paragraphs. Before this feature: api-go returned HTTP 501 on
real ingestion, nothing computed on a GPU, retrieval couldn't be
measured. Explain WHY this was the blocking item (#4 benchmark and #3
eval both need a live endpoint). Link the 501 stub commit/line if you
want to show the honest starting point. -->

## What I built

<!-- The topology in one diagram (ASCII or image) plus a short walkthrough
of the request path: client → api-go (k3s) → Tailscale → vLLM (Vast.ai)
→ Qdrant. Then the pieces you actually touched:
- vLLM serving setup on the rented GPU (model, dimension, launch flags)
- Tailscale tailnet + ACL (who can reach what)
- api-go changes: env config, embeddings client, chunk→embed→upsert,
  query→embed→search, /ready dependency check, dimension guard
- Prometheus metrics (vllm_request_duration_seconds, vllm_request_errors_total)
Keep code snippets short — link to files instead of pasting walls. -->

## Key decisions

<!-- Bullet list, each linking to where it's argued properly:
- Why an external GPU host at all → ADR-002
- Why Tailscale over a public endpoint or Cloudflare Tunnel → ADR-003
- Which embedding model + dimension, and why (size/cost/quality trade-off)
- How the model/dimension mismatch hard rule is enforced (not just documented)
One sentence of rationale each — the ADRs carry the full argument. -->

## What broke along the way

<!-- THE MOST VALUABLE SECTION — do not skip it, do not sanitize it.
Every real integration has 2-5 of these. For each: symptom → wrong
hypothesis (if any) → actual cause → fix → what would have caught it
earlier. Examples of the kind of thing that belongs here: Tailscale ACL
blocking the port, vLLM binding to the wrong interface, dimension
mismatch producing silent garbage, GPU instance running out of VRAM on
model load, Vast.ai instance reclaimed mid-session. -->

## Numbers

<!-- Fill with MEASURED values only; delete rows you didn't measure.
| Metric | Value | How measured |
|---|---|---|
| GPU class / $ per hour | | Vast.ai invoice |
| Embedding model / dimension | | /v1/models + config |
| Embedding latency p50/p95 (single query) | | smoke test / Grafana |
| End-to-end ingest of demo-docs (N docs, M chunks) | | timed run |
| Total GPU spend for this feature | | Vast.ai billing |
The deep latency/throughput analysis belongs to #4 — this table only
proves the path is real and shows cost discipline. -->

## What I'd do differently

<!-- 2-4 honest bullets. E.g. "should have tested the dimension guard
before the first real ingest", "spent too long on X because I didn't
read Y". This section signals seniority more than the success story. -->

## What's next

<!-- One short paragraph: #4 benchmark now has a live endpoint to
measure; #3 eval harness can produce dense-vs-lexical numbers; reranker
(#1) after that. Link the book-issues. -->
