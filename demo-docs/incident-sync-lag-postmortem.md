# Incident Postmortem — Knowledge Ingestion Lag After GPU Host Restart

## Summary

On 2026-03-07, document ingestion lagged for 1 hour 47 minutes after the external GPU host restarted unexpectedly. The `vLLM` embedding endpoint became unavailable during host boot and model warm-up, while the ingestion worker kept retrying and building backlog. Retrieval for already indexed content remained mostly available, but newly uploaded content did not become searchable within the expected time window.

## Severity

**SEV-2**

Reason:

- public retrieval API stayed mostly available
- existing indexed knowledge continued to serve
- new content ingestion missed freshness expectations
- there was no permanent data loss

## Impact

User-visible impact:

- newly uploaded documents did not appear in search results
- ingestion latency increased from minutes to nearly two hours
- admin operators saw growing backlog and repeated embedding failures

Non-impact:

- existing Qdrant data remained intact
- Qdrant availability was not degraded
- Grafana remained available
- public `/health` endpoint still returned success

## What happened

The platform uses an external GPU host to serve `vLLM`, which provides embeddings for ingestion. That host rebooted after a package update and came back slowly because the model-serving container required GPU initialization and model load time before the embedding API was usable.

The ingestion worker in `api-go` retried failed embedding calls, but the retry policy was too weak operationally:

- no circuit breaker
- no explicit dependency status surfaced in dashboards
- backlog growth was visible only indirectly
- alerting did not fire early enough

As a result, ingestion lag accumulated until the GPU host and `vLLM` endpoint became healthy again.

## Detection

The incident was detected by manual observation, not by the ideal alert path.

Signals that eventually exposed the issue:

- ingestion job logs showed repeated timeouts to the embedding endpoint
- the count of newly indexed chunks in Qdrant stopped increasing
- a manual smoke test for a freshly uploaded document returned no hits
- GPU host SSH check showed recent reboot time

Alerting gap:

- there was no dedicated alert on `embedding_request_failures_total`
- there was no alert on ingestion age or backlog age
- `/ready` on the public API remained green because search on already-indexed content still worked

## Timeline

All times UTC.

- **09:12** — GPU host restarted after unattended package update
- **09:14** — `vLLM` container started but model still loading
- **09:18** — first ingestion embedding timeout observed in `api-go` logs
- **09:27** — first fresh-document smoke test failed to find new content
- **09:33** — operator checked Qdrant and confirmed collection size was not growing
- **09:40** — operator SSHed to GPU host and found recent reboot
- **09:46** — `vLLM` process confirmed up but still warming model weights
- **10:08** — embedding endpoint began returning intermittent successes
- **10:21** — ingestion backlog started draining
- **10:59** — all queued chunks successfully embedded and upserted
- **11:07** — post-incident validation completed

## Customer-facing symptoms

A user querying for older documents got valid results.

A user querying for a document uploaded during the incident window got one of two outcomes:

- no relevant results
- results from older, semantically adjacent chunks

That made the issue feel like poor retrieval quality instead of clear ingestion failure.

## Root cause

Primary root cause:

- the external GPU host restarted, causing temporary loss of the `vLLM` embeddings endpoint

Contributing causes:

- `vLLM` warm-up time was not included in dependency readiness expectations
- ingestion worker lacked durable queue visibility and explicit backlog metrics
- no early alert existed for embedding endpoint unavailability
- architecture depends on a single GPU host with no failover path

## Why retrieval did not fully fail

The retrieval path for already indexed data still worked because:

- Qdrant remained healthy
- existing embeddings were already present
- only the generation of **new** embeddings was blocked

This distinction matters. The platform had a **freshness incident**, not a full search outage.

## What went well

- existing retrieval stayed online
- Qdrant data was not at risk
- logs were detailed enough to reveal the failing dependency
- issue was recoverable without data re-ingestion from scratch

## What went poorly

- alerting did not detect freshness failure quickly
- operators had no single dashboard panel showing “ingestion lag”
- the public API health model was too optimistic for ingestion workflows
- external GPU host restart behavior was not documented as an expected operational event

## Immediate remediation

Actions taken during the incident:

- verified Qdrant health and ruled out storage failure
- verified recent reboot on GPU host
- checked `vLLM` process and model load state
- let retries continue instead of forcing a full ingestion restart
- ran smoke tests until new documents became searchable again

## Corrective actions

## Completed

- documented the dependency chain `ingester -> vLLM -> Qdrant`
- added manual smoke test for newly uploaded content to the deploy checklist
- added host uptime check to incident triage notes

## Planned

1. **Add embedding endpoint availability alert**
   - alert on repeated 5xx or timeout rate
2. **Expose ingestion lag metric**
   - measure age of oldest unprocessed ingestion item
3. **Expose backlog depth**
   - even if queue is simple, operators need visibility
4. **Add startup grace and warm-up status for GPU host**
   - do not assume process started equals service ready
5. **Document maintenance window behavior**
   - avoid silent host restarts during active demo windows
6. **Consider fallback embedding provider**
   - slower and more expensive, but useful for resilience

## Prevention ideas

Medium-term architecture improvements:

- move ingestion to a dedicated worker with clearer queue semantics
- add circuit breaker and exponential backoff with operator-visible state
- introduce warm standby GPU endpoint or alternate provider
- mark admin ingestion flows degraded when the embedding dependency is unavailable

## Lessons learned

This incident was not caused by “vector database instability” or “bad chunking.” It was a dependency-availability issue on the embeddings side.

The main lesson is simple:

**In a retrieval system, freshness is a separate reliability dimension from serving availability.**

A system can answer queries and still be degraded if new knowledge cannot enter the index.

## Follow-up actions and owners

- Add `embedding_request_failures_total` metric — **Owner: platform**
- Add `ingestion_oldest_pending_seconds` metric — **Owner: platform**
- Add dashboard panel for fresh document searchability SLI — **Owner: observability**
- Add GPU host reboot/warm-up runbook — **Owner: infra**
- Evaluate low-cost fallback embedding path — **Owner: architecture**

## Final status

Resolved.

No permanent data loss occurred. The platform returned to normal behavior once the GPU host and `vLLM` embedding endpoint were fully healthy and the backlog drained.
