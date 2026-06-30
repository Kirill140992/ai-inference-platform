# AI Inference Platform

> Self-hosted **RAG inference platform** on Kubernetes — built to demonstrate the full **MLOps lifecycle** (serve → retrieve → evaluate → optimize → observe) on top of production-grade DevSecOps and GitOps practices.

![Go](https://img.shields.io/badge/Go-1.24-00ADD8) ![Kubernetes](https://img.shields.io/badge/Kubernetes-k3s-326CE5) ![GitOps](https://img.shields.io/badge/GitOps-ArgoCD-EF7B4D) ![Vector%20DB](https://img.shields.io/badge/Vector%20DB-Qdrant-DC244C)

A retrieval-augmented generation platform: a Go API serves search and answers over a Qdrant vector store, with embeddings and generation from a vLLM model server on an external GPU host. The aim isn't a hyperscale stack — it's **clear engineering decisions, measured quality and performance, and honest operational boundaries.**

## Why this repo

Most infrastructure portfolios stop at "I deployed a service." This one closes the **MLOps loop with measurement**:

| Stage | What it demonstrates | Where |
|---|---|---|
| **Serve** | Go API + vLLM model serving (OpenAI-compatible) | `app/api-go`, [architecture](docs/architecture.md) |
| **Retrieve** | Dense RAG over Qdrant (reranking & hybrid search in progress) | [ingestion design](demo-docs/knowledge-ingestion-design.md) |
| **Evaluate** | Offline retrieval eval — recall@k / MRR / nDCG | [`#3`](docs/book-issues/03-offline-eval.md) |
| **Optimize** | vLLM latency / throughput / cost benchmarking | [`#4`](docs/book-issues/04-inference-benchmark.md) |
| **Observe** | Prometheus + Grafana, runbooks, real postmortems | [observability](demo-docs/observability-guide.md) |

## Architecture

![Architecture](architecture/diagram.png)

- **api-go** (Go 1.24, stateless) — retrieval + ingestion; exposes Prometheus metrics.
- **Qdrant** (stateful) — vector store; the durable source of truth for indexed knowledge.
- **vLLM** (external GPU host) — embeddings + generation over an OpenAI-compatible API.
- **k3s + ArgoCD** — GitOps delivery, App-of-Apps, drift detection and self-healing.

Design trade-offs are written up as ADRs: [why Qdrant over a managed vector DB](demo-docs/adr-001-why-qdrant.md) · [why an external GPU host](demo-docs/adr-002-why-external-gpu-host.md).

## Results & benchmarks

> Measurement-first: every number below is produced by the eval harness and load tests **in this repo**, not hand-waved. _Populated as the eval/inference work lands — see the backlog._

| Metric | Baseline | Current | How it's measured |
|---|---|---|---|
| Retrieval recall@5 | _TBD_ | _TBD_ | offline eval set ([`#3`](docs/book-issues/03-offline-eval.md)) |
| Retrieval MRR / nDCG@10 | _TBD_ | _TBD_ | offline eval set ([`#3`](docs/book-issues/03-offline-eval.md)) |
| vLLM latency p50 / p95 | _TBD_ | _TBD_ | load test ([`#4`](docs/book-issues/04-inference-benchmark.md)) |
| vLLM throughput (tok/s · max QPS) | _TBD_ | _TBD_ | load test ([`#4`](docs/book-issues/04-inference-benchmark.md)) |
| Reranker precision lift | _TBD_ | _TBD_ | before/after vs eval set ([`#1`](docs/book-issues/01-reranker.md)) |

## Engineering depth

- **GitOps delivery** — merge to `main` → ArgoCD syncs and self-heals; no external system holds cluster credentials. [CD flow](docs/ci-cd-flow.md)
- **Security shift-left** — Checkov, Trivy, Semgrep, SonarQube in CI; SOPS-encrypted secrets; Cloudflare WAF at the edge. [Security](docs/security.md)
- **Observability & on-call** — Prometheus/Grafana dashboards, [observability guide](demo-docs/observability-guide.md), [on-call checklist](demo-docs/oncall-checklist.md).
- **Runbooks & postmortems** — real failure analysis, not just happy paths: [sync-lag postmortem](demo-docs/incident-sync-lag-postmortem.md) · [RPC latency postmortem](demo-docs/rpc-latency-postmortem.md) · [Qdrant backup/restore](demo-docs/qdrant-backup-restore-runbook.md).

## Stack

- **App & AI:** Go 1.24, vLLM, Qdrant
- **Platform:** Kubernetes (k3s), ArgoCD, Helm, Terraform (AWS / Hetzner)
- **Security:** Cloudflare WAF, Checkov, Trivy, Semgrep, SonarQube, SOPS
- **Observability:** Prometheus, Grafana

## Build

```bash
cd app/api-go
go build ./...     # compiles the API
go vet ./...       # static checks
```

Running the full stack (Qdrant + vLLM + k3s) is described in the [infrastructure guide](docs/infrastructure.md). Current and planned work lives in the [roadmap](ROADMAP.md) and [feature backlog](docs/book-issues/).

## Repo map

- `app/api-go` — Go API (retrieval + ingestion)
- `k8s/`, `helm/`, `argocd/` — manifests, charts, App-of-Apps
- `terraform/` — AWS / Hetzner IaC
- `demo-docs/` — architecture, ADRs, runbooks, postmortems
- `docs/book-issues/` — current feature backlog (in-flight work)

## About

Built by **Kirill Cheremushkin** — DevOps engineer moving into **MLOps / AIOps**. This repo is an active portfolio of that transition: production-minded infrastructure plus a measured AI-serving loop.

_Contact: ‹add email / LinkedIn›_
