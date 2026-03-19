# Observability Guide

## Purpose

This guide describes what should be observed in the retrieval platform, which dashboards matter most, and which alerts are worth adding next.

The goal is not “collect every metric.” The goal is to observe the system in a way that helps answer real operational questions quickly.

## Observability goals

For this project, observability must answer five questions:

1. Is the public API up?
2. Is the API actually ready to serve search correctly?
3. Is Qdrant healthy and returning results fast enough?
4. Is the external `vLLM` dependency reachable and fast enough?
5. Is ingestion keeping the knowledge base fresh?

## Main components to observe

- `api-go`
- Qdrant
- k3s node and pod health
- external GPU host and `vLLM`
- ingestion behavior
- end-to-end retrieval quality smoke checks

## Health vs readiness

This distinction matters and should be explicit.

## Health

Health answers:

**Is the process alive enough to respond at all?**

Example use:

- process supervision
- liveness probe
- crash detection

A healthy process can still be unable to serve real search requests.

## Readiness

Readiness answers:

**Can this instance safely serve production traffic right now?**

For this project, readiness should fail when critical serving dependencies are unavailable.

At minimum, `api-go` readiness should consider:

- Qdrant connectivity
- required config loaded
- external `vLLM` dependency available if the retrieval path needs query embeddings

Qdrant itself also exposes health-related endpoints such as `/healthz`, `/livez`, and `/readyz`, while metrics are available through `/metrics` in Prometheus/OpenMetrics format.

## Core metrics by component

## api-go

The most important API metrics:

- request rate
- request duration
- error rate
- readiness status
- downstream dependency latency
- downstream dependency failures

Recommended metric families:

- `http_requests_total`
- `http_request_duration_seconds`
- `http_request_errors_total`
- `readiness_status`
- `embedding_rpc_duration_seconds`
- `embedding_rpc_failures_total`
- `qdrant_search_duration_seconds`
- `qdrant_search_failures_total`

If answer generation is enabled separately, also add:

- `generation_rpc_duration_seconds`
- `generation_rpc_failures_total`

## Qdrant

Qdrant should be observed for:

- availability
- search latency
- collection count and point count
- memory pressure
- storage usage
- restart frequency

Qdrant has built-in metrics exposure suitable for Prometheus-compatible scraping.

## k3s / Kubernetes

Cluster-level basics:

- pod restart count
- container waiting states
- node disk pressure
- node memory pressure
- PVC status
- deployment availability
- StatefulSet health

These are boring metrics until the day they save you.

## External GPU host and vLLM

This is where many hidden failures live.

Track:

- endpoint availability
- embedding request latency
- timeout count
- process restarts
- GPU memory pressure
- model warm-up time after restart
- host uptime and recent reboot

Because `vLLM` is outside k3s, these metrics must be exported or collected intentionally. Do not assume the cluster dashboards will tell the whole story.

## Ingestion

Freshness is a first-class concern.

Track:

- ingestion job success/failure count
- chunks processed per minute
- oldest pending document age
- oldest pending chunk age
- last successful ingestion timestamp
- upsert failures to Qdrant
- embedding failures during ingestion

Without these, a “working” search system may quietly stop learning new data.

## Most important dashboards

## 1. Top-level platform overview

This should be the first dashboard an operator opens.

Panels:

- API request rate
- API p50/p95/p99 latency
- API error rate
- readiness success rate
- Qdrant availability
- embedding endpoint latency
- ingestion freshness / lag

If this dashboard is cluttered, it is failing its job.

## 2. API dependency breakdown

Purpose:

- distinguish app-level latency from downstream latency

Panels:

- `embedding_rpc_duration`
- `qdrant_search_duration`
- optional generation latency
- timeout counts by dependency
- request concurrency

This dashboard is critical during “the API is slow” incidents.

## 3. Qdrant health dashboard

Panels:

- collection count
- point count
- storage usage
- pod restarts
- search latency
- error counts
- Qdrant readiness

Use this dashboard to answer:

- is the vector store alive
- is it full
- is it slow
- did it restart

## 4. GPU host / vLLM dashboard

Panels:

- endpoint success rate
- embedding p95 latency
- process restarts
- host uptime
- GPU memory utilization
- warm-up duration after restart

This dashboard exists because external dependencies hide problems.

## 5. Ingestion freshness dashboard

Panels:

- documents ingested over time
- chunks embedded over time
- chunks upserted over time
- backlog depth
- oldest pending item age
- last successful ingestion run

This dashboard prevents “everything is green, but new docs are missing” incidents.

## What to look at during a degradation

## If the API is down

Look first at:

- deployment availability
- pod status
- recent logs
- readiness failures
- node pressure

## If the API is up but slow

Look first at:

- API p95 and p99 latency
- dependency breakdown
- `vLLM` endpoint timing
- Qdrant search timing
- recent deploy history

## If search quality suddenly looks wrong

Look first at:

- ingestion freshness
- Qdrant collection point count
- embedding model version label
- payload integrity
- recent reindex or restore events

## If new documents are missing from search

Look first at:

- ingestion lag
- embedding failure rate
- Qdrant upsert failures
- GPU host availability

## Alerts worth having now

Recommended immediate alerts:

### Availability

- API readiness failing
- Qdrant readiness failing
- Qdrant pod restart spike
- external embedding endpoint unreachable

### Latency

- API p95 above threshold for sustained period
- embedding RPC p95 above threshold
- Qdrant search latency above threshold

### Freshness

- no successful ingestion in last N minutes
- oldest pending ingestion item exceeds threshold
- chunk upsert count flatlines during expected ingest window

### Storage

- Qdrant PVC usage above threshold
- PVC not Bound
- disk pressure on node hosting Qdrant

## Alerts worth adding later

As the project grows, add:

- synthetic canary query failure
- recall/quality regression signal on a fixed evaluation set
- model version mismatch between ingestion and query embedding paths
- backup age too old
- snapshot job failure

## Example SLI candidates

This project can define simple service indicators:

- successful retrieval requests as a ratio of total retrieval requests once `/search` is implemented
- p95 search latency under target
- freshness: time from document upload to searchable availability
- Qdrant availability
- embedding endpoint availability

Even if the project is small, SLIs force better thinking.

## Common observability mistakes to avoid

Avoid these traps:

- relying on `/health` only
- hiding dependency timing inside one giant API latency graph
- not tracking ingestion freshness
- assuming Kubernetes green means user experience is green
- not instrumenting the external GPU host because it is “outside the cluster”

## Final recommendations

If time is limited, prioritize these first:

1. top-level overview dashboard
2. dependency latency breakdown
3. ingestion freshness panel
4. Qdrant storage / readiness panels
5. alert on embedding dependency failures

Those five pieces give the best operational return for this architecture.
