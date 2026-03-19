# Incident Postmortem — Retrieval API Latency Spike Caused by Slow Embedding RPC Path

## Summary

On 2026-03-12, p95 latency for the retrieval API increased from roughly 350 ms to 3.8 s for 34 minutes. The issue was caused by elevated latency on the `api-go -> vLLM` RPC path, which in turn delayed query embedding generation before Qdrant search could even begin.

The incident did not fully take the API down, but it made the service feel slow and unreliable.

## Severity

**SEV-2**

Reason:

- search requests still returned eventually for many clients
- error rate increased but did not become total outage
- user experience degraded materially

## Impact

Observed impact:

- p95 retrieval latency rose to 3.8 s
- p99 latency exceeded 6 s
- timeout rate increased for clients with aggressive request deadlines
- some readiness probes remained green even while user experience was poor

Unaffected or mostly unaffected:

- Qdrant point count and storage state
- Grafana availability
- already ingested data correctness

## System path involved

Request path:

1. client sends query to `api-go`
2. `api-go` calls external `vLLM` endpoint for query embedding
3. `api-go` submits vector to Qdrant
4. Qdrant returns top matches
5. `api-go` assembles response

The key point is that Qdrant was not the main bottleneck during this incident. The latency was front-loaded in the embedding RPC step.

## Detection

This incident was detected through latency dashboards in Grafana and manual smoke tests.

Observed signals:

- `http_request_duration_seconds` on `api-go` increased sharply
- request rate stayed normal, so the issue was not a traffic surge
- Qdrant latency panel stayed comparatively stable
- `vLLM` request duration grew and became highly variable

Alert behavior:

- a high-latency alert fired
- no dependency-specific alert identified the external embedding path as the culprit
- on-call still needed manual correlation between API latency and embedding RPC latency

## Timeline

All times UTC.

- **14:03** — p95 latency on `/search` starts climbing
- **14:06** — high-latency alert fires for public API
- **14:10** — operator checks pod health; pods are Running and Ready
- **14:12** — Qdrant dashboard checked; search latency appears normal
- **14:16** — `api-go` logs show slower-than-normal embedding request timings
- **14:19** — operator confirms no unusual deployment or rollout happened in k3s
- **14:23** — GPU host checked; host is healthy but under high GPU memory pressure after model reload tuning change
- **14:27** — `vLLM` process restarted with previous serving parameters
- **14:31** — latency begins to recover
- **14:37** — p95 falls below 800 ms
- **14:42** — p95 returns near baseline
- **14:48** — incident review starts

## Root cause

Primary root cause:

- `vLLM` query embedding latency increased significantly after a serving parameter change on the GPU host

Contributing factors:

- dependency-specific metrics were not prominent enough in the main dashboard
- readiness checks only captured coarse availability, not degraded performance
- the external GPU path is outside the k3s scheduling domain, so cluster health looked normal while user experience degraded

## Why alerts were incomplete

The platform had a general API latency alert, which was good enough to show that something was wrong.

But the alert model was incomplete because it did not answer:

- is the delay inside `api-go` itself
- is the delay in Qdrant
- is the delay in the external embedding RPC
- is the delay caused by answer-generation rather than retrieval

So the alert was useful for detection, but weak for diagnosis.

## What went well

- API latency alert fired quickly
- Qdrant metrics helped rule out the vector store early
- rollback of model-serving parameters restored performance without data loss
- deployment history in k3s helped exclude a bad API rollout

## What went poorly

- the main dashboard emphasized app-level symptoms more than dependency timing
- readiness remained green despite degraded request experience
- no SLO-style alert existed for the embedding RPC dependency itself
- the external serving layer was under-instrumented compared with the in-cluster components

## Key diagnostic steps

These were the highest-value checks:

1. Compare API latency against Qdrant latency
2. Check `api-go` logs for downstream call timing
3. Verify there was no fresh `api-go` deploy
4. Inspect GPU host status and recent model-serving changes
5. Re-run a controlled smoke query before and after rollback

## Remediation

During the incident:

- reverted the recent `vLLM` serving parameter change
- restarted the model-serving process
- validated reduced latency on a fixed smoke query
- kept the public API deployment unchanged

## Corrective actions

## Completed

- restored prior known-good `vLLM` serving configuration
- added note in change log for GPU host tuning changes

## Planned

1. **Add downstream latency breakdown to the main dashboard**
   - `embedding_rpc_duration`
   - `qdrant_search_duration`
   - `response_assembly_duration`
2. **Add alert on dependency latency, not only API latency**
3. **Track timeout budget consumption**
   - so operators can see how much latency is spent before search even starts
4. **Create a synthetic query monitor**
   - fixed query run every minute against full path
5. **Define degraded-readiness policy**
   - decide whether very high downstream latency should fail readiness or only alert

## Why Qdrant was not blamed

This matters because vector databases often get blamed first in retrieval incidents.

Evidence that Qdrant was not the root cause:

- Qdrant service stayed reachable
- Qdrant search latency stayed near normal
- the latency spike appeared before the vector search step
- restoring `vLLM` settings fixed the issue without touching Qdrant

## Lessons learned

The user experiences one API, but the reliability story has multiple internal latency budgets.

In this architecture, query latency is dominated by:

- query embedding generation
- vector search
- optional answer generation

If the first stage slows down, the whole system looks broken even if the database is healthy.

## Follow-up action items

- Add dependency latency panels to the top-level dashboard — **Owner: observability**
- Add alert for `embedding_rpc_p95_seconds` — **Owner: platform**
- Add change log discipline for GPU host tuning — **Owner: infra**
- Create synthetic canary query — **Owner: platform**
- Review readiness semantics for degraded downstreams — **Owner: architecture**

## Final status

Resolved.

User-facing latency returned near baseline after reverting the `vLLM` tuning change. No data corruption occurred.
