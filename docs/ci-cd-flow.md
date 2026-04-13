# Continuous Delivery & GitOps Flow

The platform implements a **"Pull-based" GitOps** architecture using **ArgoCD** for Kubernetes deployments. This replaces traditional Push-based pipelines (which require storing high-privileged Kubernetes credentials inside external CI platforms) and significantly reduces the attack surface.

## The GitOps Paradigm (ArgoCD)

* **Zero External Credentials:** ArgoCD runs *inside* the Kubernetes cluster. It polls the GitHub repository via secure HTTPS. The cluster pulls the desired state, meaning no external CI system has direct write access to the Kubernetes API.
* **App of Apps Pattern:** A single Root Application acts as the "Director," securely managing and cascading configurations to child applications (`qdrant-vector-db`, `api-go-backend`, `platform-infrastructure`).
* **Automated Drift Detection & Self-Healing:** ArgoCD constantly reconciles the live cluster state with the Git repository. If an operator or attacker manually modifies a resource directly in the cluster (e.g., via `kubectl edit`), ArgoCD instantly detects the configuration drift and automatically overwrites it, ensuring the cluster strictly mirrors the code-reviewed state in Git.

## Deployment Flow
1. Code changes or Helm value updates are merged into the `main` branch.
2. ArgoCD detects the new commit in the repository.
3. ArgoCD automatically pulls the new manifests and synchronizes the cluster state without manual `kubectl` intervention.

### ArgoCD App of Apps Dashboard
![ArgoCD App of Apps Dashboard](../architecture/argocd-dashboard.png)

### Automated Sync & Self-Healing Enabled
![ArgoCD Automated Sync & Self-Heal](../architecture/argocd-autosync.png)