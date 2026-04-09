# Platform Roadmap & Future Architecture

The AI Inference Platform is continuously evolving. The roadmap is divided into two phases: near-term operational improvements to stabilize the current architecture, and long-term strategic initiatives to align with Enterprise DevSecOps, SRE, and FinOps standards.

---

## Phase 1: Near-Term Operations & Scalability
*Focus: Hygiene, GitOps, and Architectural Bottlenecks.*

### 1. GitOps & Observability Baseline
* **ArgoCD Integration:** Transition from manual Kubernetes manifests application to a strict GitOps pull-based deployment model using ArgoCD.
* **Telemetry Dashboards:** Develop and provision comprehensive Grafana dashboards (as Code) to visualize existing Prometheus metrics (e.g., API latency, Qdrant vector search duration, chunking errors).

### 2. Asynchronous Task Processing
* **Objective:** Prevent HTTP 504 timeouts during the ingestion of massive document payloads.
* **Implementation:** Introduce **Redis** to the k3s cluster and refactor the `api-go` ingestion handler using the `asynq` library. The API will return `HTTP 202 Accepted` immediately, offloading the heavy document chunking and embedding generation to background worker goroutines.

### 3. Zero Trust External Networking (ZTNA)
* **Objective:** Secure the communication between the in-cluster Go API and the external Vast.ai GPU Host.
* **Implementation:** Deploy a **Tailscale** (or Cloudflare Tunnels) mesh network. This ensures the vLLM endpoint is never exposed to the public internet, routing all inference traffic through an encrypted P2P WireGuard tunnel.

### 4. Enterprise Secrets Management
* **Objective:** Move beyond static Git-encrypted secrets (SOPS).
* **Implementation:** Deploy **HashiCorp Vault**. Transition from environment variables to dynamic secrets injection via the Vault Agent Injector and Kubernetes Service Account (KSA) native authentication.

---

## Phase 2: Strategic Enterprise Features
*Focus: AI-Native Operations, Advanced Security, and Cost Management.*

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