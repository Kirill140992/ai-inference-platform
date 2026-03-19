# On-Call Checklist

## Purpose

This checklist is the fast triage guide for the retrieval platform.

It is intentionally short and operator-focused. The goal is to reduce time wasted in the first 5–10 minutes of an incident.

## Core rule

Before making changes, answer three questions:

1. Is the problem **availability**, **latency**, or **freshness**?
2. Is the failure in **api-go**, **Qdrant**, or the **external GPU / vLLM** path?
3. Is user traffic broken, or only ingestion / newly indexed content broken?

Do not jump straight into restarting random pods.

## First 2-minute triage

Run these checks first:

```bash
kubectl -n ai-platform get pods -o wide
kubectl -n ai-platform get deploy,statefulset,svc,pvc
kubectl -n ai-platform get events --sort-by=.lastTimestamp | tail -n 30
curl -fsS http://<API_BASE_URL>/health
curl -fsS http://<API_BASE_URL>/ready
```

Classify the incident:

- API down
- API up but not ready
- API slow
- Qdrant empty or missing data
- GPU endpoint unavailable
- new documents missing from search

## If the API is not responding

Check in this order.

### 1. Is the deployment healthy

```bash
kubectl -n ai-platform get deployment api-go
kubectl -n ai-platform get pods -l app=api-go -o wide
kubectl -n ai-platform describe deployment api-go
```

### 2. Check logs

```bash
kubectl -n ai-platform logs deployment/api-go --tail=200
```

Look for:

- startup failures
- bad config
- dependency timeouts
- panic or crash patterns

### 3. Check service / ingress path

```bash
kubectl -n ai-platform get svc
kubectl -n ai-platform describe svc api-go
```

### 4. Check recent rollout

```bash
kubectl -n ai-platform rollout history deployment/api-go
```

If the issue started right after deploy, rollback is a strong candidate.

## If `/ready` is failing

This usually means the process is alive but a critical dependency is not healthy enough.

Check:

```bash
curl -v http://<API_BASE_URL>/ready
kubectl -n ai-platform logs deployment/api-go --tail=200
```

Then verify dependencies:

### Qdrant

```bash
kubectl -n ai-platform get pods -l app=qdrant -o wide
kubectl -n ai-platform logs statefulset/qdrant --tail=200
kubectl -n ai-platform get pvc
```

### External embedding endpoint

```bash
curl -v http://<VLLM_URL>/health
```

If `vLLM` is external and the API depends on it for query embeddings, a failing `/ready` often points there.

## If the API is up but very slow

Do not assume the database is the problem.

Check in this order:

1. API logs for downstream timing
2. embedding endpoint latency
3. Qdrant search latency
4. recent deploy or config change
5. GPU host status

Useful checks:

```bash
kubectl -n ai-platform logs deployment/api-go --tail=200
curl -w '\nconnect=%{time_connect} total=%{time_total}\n' -o /dev/null -s http://<API_BASE_URL>/ready
curl -w '\nconnect=%{time_connect} total=%{time_total}\n' -o /dev/null -s http://<VLLM_URL>/health
```

If the API is slow but Qdrant is healthy, suspect the external embedding path.

## If Qdrant looks empty

Symptoms:

- search returns no hits
- collection count or point count looks wrong
- API works but results are empty or obviously degraded

Check:

```bash
kubectl -n ai-platform get pvc
kubectl -n ai-platform exec -it statefulset/qdrant -- ls -lah /qdrant/storage
curl http://<QDRANT_URL>:6333/collections
curl http://<QDRANT_URL>:6333/collections/knowledge-base
```

Questions to answer:

- did the pod restart and reattach the same PVC
- did the PVC get lost or recreated
- was there a restore or reindex recently
- is the expected collection name present

If collection is missing and PVC was lost, move immediately to backup/restore procedure.

## If the GPU endpoint is unavailable

Symptoms:

- ingestion stalls
- query requests fail or slow down before search
- `/ready` may fail depending on policy

Check:

```bash
curl -v http://<VLLM_URL>/health
```

Then verify host reachability:

```bash
ping <VLLM_HOST>
ssh <VLLM_HOST>
```

Once on host, check:

- recent reboot
- `vLLM` process status
- GPU memory pressure
- container restart count
- model warm-up still in progress

Do not restart `api-go` first if the external dependency is clearly down.

## If new documents are missing from search

This is a freshness problem, not necessarily a serving outage.

Check:

- ingestion logs
- embedding failures
- Qdrant upsert success
- collection size growth
- timestamp of last successful ingestion

Commands:

```bash
kubectl -n ai-platform logs deployment/api-go --tail=300 | grep -i ingest
curl http://<QDRANT_URL>:6333/collections/knowledge-base
```

A green API with stale ingestion is still a degraded system.

## Quick rollback decision

Rollback `api-go` if:

- incident started immediately after deploy
- logs point to config or code regression
- dependencies are healthy but behavior changed

Do **not** rollback `api-go` just because search is slow if the real issue is on the GPU host.

## What not to do in the first 10 minutes

Avoid these mistakes:

- deleting Qdrant pod before checking PVC
- wiping collection before confirming backup state
- restarting everything at once
- making config changes without noting the previous state
- confusing “health” with “ready”
- assuming fresh-document issues are search-ranking problems

## Escalation map for this project

Think in domains:

- **api-go / deployment issue** — app rollout, config, readiness logic
- **Qdrant issue** — stateful storage, collection health, PVC, restore
- **GPU / vLLM issue** — external dependency, warm-up, host reboot, latency
- **network issue** — DNS, routing, firewall between cluster and GPU host

## End-of-triage checklist

Before deeper debugging, you should know:

- are pods Running
- is `/health` green
- is `/ready` green
- is Qdrant reachable
- is `vLLM` reachable
- is the issue old data, new data, or all traffic
- was there a recent deploy

If you do not know those seven things yet, you are not past triage.
