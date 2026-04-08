#  AI Inference Platform on Kubernetes

Enterprise-grade self-hosted AI knowledge base platform built on Kubernetes, Go, and Qdrant. 
This project serves as a comprehensive showcase of modern **DevSecOps**, **Zero Trust architecture**, and **Infrastructure as Code (IaC)** practices.

##  Project Documentation

The repository is structured into distinct architectural domains. Click on any section below to dive into the technical details, configurations, and dashboards:

*  **[Architecture & RAG Implementation](./docs/architecture.md)**
  Overview of the Go-based API, Qdrant vector database integration, and the AI inference pipeline logic.

*  **[DevSecOps & Edge Security](./docs/security.md)**
  Comprehensive overview of the security posture, including CI/CD Shift-Left gates, Infrastructure hardening, and Cloudflare Edge protection.

*  **[Infrastructure as Code](./docs/infrastructure.md)**
  How the AWS environment is provisioned using Terraform, and Kubernetes cluster configuration.

*  **[Observability & Monitoring](./docs/monitoring.md)**
  Metrics, logging, and runtime security alerts using Prometheus, Grafana, and Falco.

## Stack
- **Infrastructure:** AWS, Kubernetes (k3s), Terraform
- **App & AI:** Go, vLLM, Qdrant Vector DB
- **Security:** Cloudflare (WAF/Bot Management), Checkov, Trivy, Semgrep, SonarQube, SOPS
- **Observability:** Prometheus, Grafana

---
*Maintained by Kirill Cheremushkin*