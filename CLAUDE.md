# ai-inference-platform — CLAUDE.md

Standing brief for Claude Code. Self-hosted RAG platform; this repo is also my **portfolio for DevOps → MLOps/AIOps hiring**. Read this at the start of every session.

## What this is

Stack: **k3s**, `api-go` (Go 1.24), **Qdrant** (vector DB), **vLLM** on an external GPU host (OpenAI-compatible), Prometheus/Grafana, GitOps via **ArgoCD**. Strong DevSecOps layer (Cloudflare WAF, Checkov/Trivy/Semgrep/SonarQube, SOPS).

## Goal (what we build toward)

By **~early September** (hiring season): close the must-ship features + English writeups so the repo demonstrably shows the full MLOps loop **serve → retrieve → evaluate → optimize → observe** on top of the existing DevSecOps/IaC.

## What we want to do now (backlog, prioritized)

Full specs in `docs/book-issues/`. Execution order: **#0 → #4 → #3 → #1 → #2** (cut from the tail under pressure, not the head):

0. **#0 Bring up Vast.ai vLLM + Tailscale + wire embeddings** — *must, blocks everything*. **Start here.** The embeddings/vLLM backend is currently a stub (`app/api-go/main.go` ~L453); nothing computes on the GPU yet. This completes the core RAG path so the rest can be measured. (ROADMAP #3 / ADR-002.)
1. **#4 Inference benchmark + vLLM optimization** — *must*. Closest to DevOps; **needs #0** (a live vLLM endpoint to measure). Book: Inference Optimization.
2. **#3 Offline eval harness** (recall@k / MRR / nDCG) — *must*. Rarest market signal; the measuring stick for everything else. **Needs #0.** Book: Evaluation.
3. **#1 Reranker** — *should*. Clean before/after when paired with eval. Book: RAG & Agents.
4. **#2 Hybrid (lexical + dense) search** — *defer if time is short*. Overlaps the "improved retrieval" story. Book: RAG & Agents.
5. **Writeup (English)** per feature — **not optional**. Recruiters read the repo, not run it. Doubles as IELTS practice.

## Definition of done (every feature)

- Behind a config toggle (don't break the baseline path).
- Prometheus metric + a Grafana panel.
- A measured **before/after** (use the eval harness once it exists).
- Short ADR in `demo-docs/adr-NNN-*.md`.
- `go test ./...` passes; `security-pipeline` CI green; **no plaintext secrets**.

## Architecture (quick map)

- **api-go** (stateless, Go 1.24, single `app/api-go/main.go` today): retrieval + ingestion, OpenAI-compatible calls to vLLM, exposes Prometheus metrics.
- **Qdrant** (stateful, PVC): vectors + chunk payloads + metadata. The only durable data layer (Git is source-of-truth #1, Qdrant PVC is #2).
- **vLLM** (external GPU host = **Vast.ai**, outside k3s): embeddings + optional generation. **Most failure-prone path:** `api-go → Tailscale → Vast.ai vLLM`.
- **Hard rule:** the same embedding model **and** vector dimension must be used across ingestion and query, or retrieval silently breaks.
- **Topology:** k3s runs **locally (homelab)**; vLLM runs on a **rented Vast.ai GPU** reached over a private **Tailscale** tunnel. ArgoCD in local k3s still pulls from GitHub.
- **Current status:** api-go ↔ Qdrant works locally; the **vLLM/embeddings backend is still a stub** (`main.go` ~L453) — wiring it is **#0**.

## Commands

- Build: `cd app/api-go && go build ./...`
- Run locally: `cd app/api-go && go run .`
- Test / vet / fmt: `go test ./...` · `go vet ./...` · `gofmt -l .`
- Container: `docker build app/api-go`
- No Makefile yet — fine to add one with `bench` / `eval` / `test` targets when we start #4.

## Conventions

- Branch **main**. Small PRs, one feature each; reference the matching `docs/book-issues/` item.
- **Deploy = GitOps.** Merge to `main` → ArgoCD syncs and self-heals. **Do NOT `kubectl apply`/`edit` the cluster by hand** — drift gets reverted; change Git instead.
- **Secrets via SOPS** (`.sops.yaml`, `offline-secrets/`). Never commit plaintext secrets.
- CI `.github/workflows/security-pipeline.yaml` (Checkov/Trivy/Semgrep/SonarQube) must stay green.
- Writeups and ADRs in **English**.
- Context docs to read first: `demo-docs/architecture-overview.md`, `demo-docs/knowledge-ingestion-design.md`, `docs/architecture.md`, `ROADMAP.md`.

## Working agreement (pace — this is part of the plan)

- ~2 focused blocks/week, **not all weekend**; one rest day stays protected. Quality > volume.
- Practice > theory: read the relevant book chapter, then straight into code.
- **Don't gold-plate:** must/should core + writeup → stop. Flag scope creep instead of adding features.
