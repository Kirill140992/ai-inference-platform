# Deploy Runbook

## Purpose

This runbook describes how to deploy a new version of `api-go`, validate rollout success, and roll back when needed.

This project intentionally uses a simple deployment flow:

- build a new container image
- push the image to a registry
- update the tag in the Kubernetes manifest
- apply manifests
- verify rollout and application health

That simplicity is fine for a pet project as long as the procedure is explicit and repeatable.

## Assumptions

- k3s cluster is reachable through `kubectl`
- namespace is `ai-platform`
- deployment name is `api-go`
- container name inside the deployment is `api-go`
- image registry is available
- manifests are stored in Git

Example repository layout:

```text
.
├── api-go/
├── k8s/
│   ├── namespace.yaml
│   ├── api-go-deployment.yaml
│   ├── api-go-service.yaml
│   ├── qdrant-statefulset.yaml
│   └── grafana-deployment.yaml
└── README.md
```

## Preconditions before any deploy

Before pushing a new version, confirm:

- you are on the correct Git branch
- working tree is clean or intentionally dirty
- the new image tag is known
- the cluster context points to the correct environment
- Qdrant is healthy
- the external `vLLM` endpoint is reachable if embeddings or generation are enabled

Recommended checks:

```bash
git status
git branch --show-current
kubectl config current-context
kubectl -n ai-platform get pods
kubectl -n ai-platform get svc
curl -fsS http://<API_BASE_URL>/health
curl -fsS http://<API_BASE_URL>/ready
```

## Standard deployment flow

## Step 1 — Pull the latest repository state

If the deployment host also stores manifests locally:

```bash
git pull origin main
```

If you deploy from a feature or release branch, use that branch explicitly.

## Step 2 — Build and push the image

From the repository root or the API directory:

```bash
export TAG=2026-03-18-1
docker build -t ghcr.io/example/api-go:${TAG} ./api-go
docker push ghcr.io/example/api-go:${TAG}
```

Tagging rule for this project:

- use a human-readable tag for demos and runbooks
- optionally also push a commit-SHA tag for traceability

Example dual tagging:

```bash
export TAG=$(git rev-parse --short HEAD)
docker build -t ghcr.io/example/api-go:${TAG} ./api-go
docker push ghcr.io/example/api-go:${TAG}
```

## Step 3 — Update the deployment manifest

Edit the image tag in `k8s/api-go-deployment.yaml`.

Example snippet:

```yaml
containers:
  - name: api-go
    image: ghcr.io/example/api-go:2026-03-18-1
```

If you prefer a one-liner:

```bash
sed -i.bak 's#ghcr.io/example/api-go:.*#ghcr.io/example/api-go:'"${TAG}"'#' k8s/api-go-deployment.yaml
```

Review the change:

```bash
git diff -- k8s/api-go-deployment.yaml
```

## Step 4 — Apply manifests

Apply the updated deployment manifest:

```bash
kubectl apply -f k8s/api-go-deployment.yaml
```

If you want to apply the whole stack:

```bash
kubectl apply -f k8s/
```

For small systems, applying the whole directory is acceptable, but applying only the changed manifest lowers accidental blast radius.

## Step 5 — Watch rollout

Verify that Kubernetes accepted the new template:

```bash
kubectl -n ai-platform rollout status deployment/api-go --timeout=180s
```

Also inspect the ReplicaSet transition:

```bash
kubectl -n ai-platform get rs
kubectl -n ai-platform get pods -l app=api-go -w
```

## Step 6 — Validate readiness and smoke test

After rollout completes, verify that the application is actually healthy.

### Check pod state

```bash
kubectl -n ai-platform get pods -l app=api-go -o wide
```

### Check recent logs

```bash
kubectl -n ai-platform logs deployment/api-go --tail=200
```

### Check health endpoint

```bash
curl -fsS http://<API_BASE_URL>/health
```

### Check readiness endpoint

```bash
curl -fsS http://<API_BASE_URL>/ready
```

### Run a smoke test request

Current implemented smoke test:

```bash
curl -fsS http://<API_BASE_URL>/collections
```

Planned retrieval smoke test once `/search` is implemented:

```bash
curl -X POST http://<API_BASE_URL>/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"what does the platform store in qdrant?","top_k":3}'
```

Success criteria:

- rollout completed
- no CrashLoopBackOff
- `/health` returns success
- `/ready` returns success
- current smoke request returns valid JSON

## Alternative deployment method: set image directly

If you need a quick operational change without editing the manifest first:

```bash
kubectl -n ai-platform set image deployment/api-go \
  api-go=ghcr.io/example/api-go:${TAG}
```

Then verify:

```bash
kubectl -n ai-platform rollout status deployment/api-go
```

Important: this changes live cluster state first. To avoid config drift, update the manifest in Git immediately afterward.

## Rollback procedure

Rollback is needed when any of the following happen after deployment:

- pods do not become Ready
- `/ready` fails
- search requests error out
- latency regresses sharply
- logs show dependency or configuration failures

## Fast rollback using rollout history

Check rollout history:

```bash
kubectl -n ai-platform rollout history deployment/api-go
```

Rollback to the previous revision:

```bash
kubectl -n ai-platform rollout undo deployment/api-go
```

Wait for rollback completion:

```bash
kubectl -n ai-platform rollout status deployment/api-go --timeout=180s
```

Validate again:

```bash
curl -fsS http://<API_BASE_URL>/ready
kubectl -n ai-platform logs deployment/api-go --tail=200
```

## Rollback by restoring previous image tag in Git

If you manage manifests declaratively, restore the previous known-good image tag in `k8s/api-go-deployment.yaml`, commit the change, and re-apply:

```bash
git checkout <known-good-commit> -- k8s/api-go-deployment.yaml
kubectl apply -f k8s/api-go-deployment.yaml
kubectl -n ai-platform rollout status deployment/api-go
```

This method is slower but cleaner because it keeps Git aligned with the cluster.

## Common failure cases during deploy

## New pods stuck in ImagePullBackOff

Check:

```bash
kubectl -n ai-platform describe pod <pod-name>
```

Typical causes:

- wrong tag
- image was not pushed
- registry auth secret missing

## Pods start but never become Ready

Check:

```bash
kubectl -n ai-platform logs <pod-name> --previous
kubectl -n ai-platform logs <pod-name>
curl -fsS http://<pod-or-service>/ready
```

Typical causes:

- Qdrant unreachable
- `vLLM` endpoint misconfigured
- startup migration or config validation fails

## Rollout completed but app behavior is broken

This is the most dangerous case because Kubernetes thinks the deploy succeeded.

Check:

- smoke query response quality
- Qdrant connectivity
- `vLLM` connectivity
- logs for timeout spikes
- Grafana latency panels

## Post-deploy verification checklist

Use this exact checklist after every rollout:

- deployment revision changed
- desired replicas match available replicas
- old ReplicaSet scaled down
- `/health` is green
- `/ready` is green
- one real retrieval query succeeds
- Qdrant collection count looks sane
- no unexpected latency spike
- no error burst in logs for the first 5–10 minutes

## Emergency change policy for this project

Because this is a pet project, some changes may be made manually during iteration. That is acceptable only if:

- the live change is documented
- the matching Git change is made immediately after
- rollback path is known before applying the change

Never rely on memory for the last known-good image tag.

## Suggested improvements

The current deploy model is intentionally simple. Strong next steps:

- automate image build and push in CI
- use immutable image tags only
- add a deployment pipeline with smoke tests
- fail readiness when Qdrant or `vLLM` is unreachable
- export dashboards and alerts as code
