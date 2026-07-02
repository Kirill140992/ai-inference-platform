# Platform Roadmap & Future Architecture

The AI Inference Platform is continuously evolving. The roadmap is divided into two phases: near-term operational improvements to stabilize the current architecture, and long-term strategic initiatives to align with Enterprise DevSecOps, SRE, and FinOps standards.

> **Status (2026-07-02):** the active, prioritized backlog now lives in `CLAUDE.md` and `docs/book-issues/` (execution order `#0 → #4 → #3 → #1 → #2`, targeting early September). Items below are annotated with their current disposition — **Done**, **In progress**, **Shelved**, or **Deferred** — rather than deleted, so the original plan stays legible. "Shelved/Deferred" means consciously parked, not forgotten: none of them block the MLOps-loop goal.

---

## Phase 1: Near-Term Operations & Scalability
*Focus: Hygiene, GitOps, and Architectural Bottlenecks.*

### 1. GitOps & Observability Baseline — **Done**
* **ArgoCD Integration:** Transition from manual Kubernetes manifests application to a strict GitOps pull-based deployment model using ArgoCD.
* **Telemetry Dashboards:** Develop and provision comprehensive Grafana dashboards (as Code) to visualize existing Prometheus metrics (e.g., API latency, Qdrant vector search duration, chunking errors).
* *Status:* ArgoCD app-of-apps manages the cluster (`argocd/`); dashboards-as-code started (`k8s/monitoring/api-go-dashboard.yaml`). New panels land with each feature per the CLAUDE.md definition of done.

### 2. Asynchronous Task Processing — **Shelved**
* **Objective:** Prevent HTTP 504 timeouts during the ingestion of massive document payloads.
* **Implementation:** Introduce **Redis** to the k3s cluster and refactor the `api-go` ingestion handler using the `asynq` library. The API will return `HTTP 202 Accepted` immediately, offloading the heavy document chunking and embedding generation to background worker goroutines.
* *Status:* current corpus size doesn't produce ingestion timeouts; adding a queue now would be premature. Revisit only if real ingestion volume outgrows the synchronous path after `#0` lands.

### 3. Zero Trust External Networking (ZTNA) — **In progress (backlog `#0`)**
* **Objective:** Secure the communication between the in-cluster Go API and the external Vast.ai GPU Host.
* **Implementation:** Deploy a **Tailscale** (or Cloudflare Tunnels) mesh network. This ensures the vLLM endpoint is never exposed to the public internet, routing all inference traffic through an encrypted P2P WireGuard tunnel.
* *Status:* being executed as `docs/book-issues/00-bringup-vast-tailscale.md`. Tailscale chosen over Cloudflare Tunnel — rationale in `demo-docs/adr-003-vast-tailscale-bringup.md`.

### 4. Enterprise Secrets Management — **Shelved**
* **Objective:** Move beyond static Git-encrypted secrets (SOPS).
* **Implementation:** Deploy **HashiCorp Vault**. Transition from environment variables to dynamic secrets injection via the Vault Agent Injector and Kubernetes Service Account (KSA) native authentication.
* *Status:* SOPS remains adequate for a single-operator project with few, rarely-rotated secrets. Revisit if operator count or rotation requirements grow.

---

## Phase 2: Strategic Enterprise Features — **Deferred**
*Focus: AI-Native Operations, Advanced Security, and Cost Management.*

> All Phase 2 items are consciously deferred until the MLOps loop (serve → retrieve → evaluate → optimize → observe) ships — see `CLAUDE.md`. None of them block that goal, and starting them earlier would be scope creep against the working agreement.

### 5. AIOps & Incident Response Automation
* **Implementation:** Integration of **HolmesGPT** to enrich Prometheus/Alertmanager notifications. When a Kubernetes alert fires, the AI agent will automatically fetch relevant pod logs and events, providing the on-call engineer with a pre-triaged root cause analysis directly in Slack.

### 6. LLM-Powered Shift-Left Security (AppSec)
* **Implementation:** Augmenting SonarQube with a custom GitHub Action powered by the **Anthropic Claude 3.5 API**. This acts as an automated AppSec engineer, performing deep-context security reviews on Pull Requests to detect logical flaws and IAM misconfigurations that traditional SAST tools miss.

### 7. Software Supply Chain Security (SLSA)
* **Implementation:** Integration of **Sigstore (Cosign)** for cryptographic signing of Docker images in the CI pipeline. Implementing a Kubernetes Admission Controller to verify image signatures before pod creation.

### 8. FinOps & AI Cost Management
* **Implementation:** Integrating **Infracost** into GitHub Actions for shift-left cost estimation on Terraform PRs. Deploying **Kubecost** within the cluster for granular namespace-level tracking of the AI infrastructure burn rate.

### 9. Distributed Tracing
* **Implementation:** Instrumenting the Go API with **OpenTelemetry (OTel)** and deploying Jaeger to visualize latency bottlenecks across the multi-component RAG pipeline.

### 10. On-Chain Knowledge-Base Integrity Anchoring
* **Objective:** Tamper-evident audit trail for the knowledge base: anchor a content hash per ingested document on the in-cluster EVM dev chain (`k8s/web3/`, Foundry/anvil), verifiable via an api-go endpoint.
* **Scope note:** the dev chain demonstrates the anchoring pattern and Go↔EVM integration, not a decentralized trust model — the ADR must state this limitation explicitly. Tracked as backlog item `#5` in CLAUDE.md; strictly after the MLOps must-ship items.