# ADR-003 — Vast.ai + Tailscale Bring-up for the Embeddings Backend

- **Status:** Proposed — topology and rationale are decided; the fields marked `TBD` below are filled in as part of `#0` (`docs/book-issues/00-bringup-vast-tailscale.md`) once the GPU host and model are actually running. This ADR is not "Accepted" until those fields have real values and Checkpoint 2 passes end-to-end.
- **Date:** 2026-07-02 (drafted ahead of execution)
- **Decision owners:** platform / architecture

## Context

ADR-002 already decided **that** `vLLM` runs on an external GPU host (Vast.ai) instead of inside k3s. This ADR covers the narrower, more operational question that ADR-002 deliberately left open:

**How does `api-go`, running in a local k3s cluster, reach `vLLM` on a rented GPU host safely — and what exactly gets rented, connected, and measured to make that real?**

This is the connectivity and bring-up decision behind `#0`, the item that currently blocks every other feature in the backlog (`#4`, `#3`, `#1`, `#2` all read a live embeddings endpoint).

## Decision

Use **Tailscale** as a private WireGuard-based mesh network between the local k3s node and the rented Vast.ai GPU host. `api-go` calls `vLLM` only over the tailnet address; `vLLM` is never bound to a public interface.

## Topology

```
client → api-go (local k3s) → Tailscale (private tunnel) → vLLM (Vast.ai GPU) → back → Qdrant (local k3s)
```

Both k3s and Qdrant stay local (homelab); only the GPU-bound inference call leaves the local network, and it leaves over a private tunnel rather than the public internet.

## Why Tailscale (vs. alternatives)

### Option A — Public HTTPS endpoint + bearer token
Rejected. Even with a strong token, this exposes `vLLM`'s HTTP surface to the public internet. For a project whose selling point is a deliberate DevSecOps/Zero-Trust posture, putting the one component with no WAF/rate-limiting in front of it (`vLLM` itself has none) directly on the public internet undercuts the story the rest of the repo tells.

### Option B — Cloudflare Tunnel
A reasonable alternative (also named in `ROADMAP.md`'s Zero Trust networking item). Not chosen for the first pass because it adds a third-party edge dependency and a second control plane (Cloudflare) to reason about for what is currently a single, simple point-to-point link between two hosts we already control. Worth revisiting if more than one external host needs to reach the cluster, or vice versa (see re-evaluation triggers).

### Option C — Manual WireGuard config
Tailscale is WireGuard under the hood, but without the manual key-exchange and config-file management overhead. For a two-node private mesh maintained by a single person, Tailscale's ACL model and `tailscale up`/`tailscale ping` workflow is simpler to operate and to explain in a writeup than hand-rolled WireGuard.

## Embedding model + vector dimension

`TBD` — recorded here once Checkpoint 1 of `#0` confirms the model via `vLLM`'s `/v1/models` endpoint.

- Embedding model: `TBD`
- Vector dimension: `TBD`
- Qdrant collection created with this dimension via the existing `/collections/init`: `TBD` (collection name / verification)

This is the field that backs CLAUDE.md's "hard rule" (model + dimension must match between ingestion and query). Once filled in, it should match whatever `EMBEDDING_MODEL` / `EMBEDDING_DIM` are set to in the `api-go` deployment, and whatever the dimension-mismatch safeguard (added as an acceptance criterion to `#0`) actually checks at startup.

## Cost

Vast.ai bills hourly while an instance runs. Discipline (see `CLAUDE.md` → "Checkpoints & risk controls"): the instance is started only during an active work block and stopped or destroyed before the session ends. A tiny local/CPU embedding model is used for wiring and development; the rented GPU is reserved for the final `#0` end-to-end pass and for `#4` benchmark runs.

- GPU class chosen: `TBD`
- Measured $/hr: `TBD`
- Rough monthly cost at the actual usage pattern (work blocks only, not always-on): `TBD`

## ACLs

- Tailscale ACL restricts the `vLLM` port to the local k3s node's tailnet identity only — no other tailnet member (if any are added later) can reach it.
- The Tailscale auth key is handled out-of-band and is never committed to git. Any `vLLM` API key follows the existing SOPS pattern (`.sops.yaml`, `offline-secrets/`) rather than being hardcoded or passed as a plain env value.

## Failure modes

- **GPU instance interrupted or reclaimed mid-session** (Vast.ai is a rentable/spot-adjacent marketplace — see ADR-002's accepted trade-offs) → `vLLM` becomes unreachable → `api-go`'s `/ready` should report not-ready once the dependency health check from `#0` Checkpoint 2 lands, rather than silently continuing to accept ingest/query traffic.
- **Tailscale connectivity drop** (ACL misconfiguration, expired auth key, tailnet issue) → same externally-visible symptom as a GPU failure (unreachable `vLLM`). Distinguish the two by re-running Checkpoint 1's standalone check (`curl http://<tailscale-ip>:8000/v1/models` from the k3s node, no `api-go` involved) — if that also fails, it's network, not code.
- **Embedding model/dimension drift** between an ingestion-time deploy and a later query-time deploy → silent retrieval degradation with no error surfaced anywhere. This is the specific failure CLAUDE.md's "hard rule" warns about; mitigated by the dimension-check acceptance criterion added to `#0` (fail loudly at startup on mismatch rather than serving garbage results).
- **vLLM process crash on the GPU host** (OOM, bad request, model load failure) without the instance itself going down → `/v1/models` may still respond while `/embeddings` fails. Health checks should hit a real inference path, not just process liveness.

## Alternatives considered

Covered above under "Why Tailscale" — Cloudflare Tunnel and manual WireGuard were the two live alternatives; both rejected for now on operational-simplicity grounds rather than being ruled out permanently.

## Consequences

### Positive
- Private, low-friction connectivity between two hosts we control, without exposing `vLLM` to the public internet.
- ACL-scoped access matches the project's Zero-Trust framing rather than contradicting it.
- Reuses a tool (Tailscale) with a short setup path, keeping `#0` scoped to two new integrations (Vast.ai, Tailscale) plus the `api-go` wiring, instead of three.

### Negative
- Adds an external dependency (Tailscale's control plane) for tailnet coordination, even though the data path itself is direct WireGuard.
- Failure modes for "GPU down" and "network down" look identical from `api-go`'s perspective unless deliberately distinguished (see Checkpoint 1's standalone verification step).
- Tailscale ACLs are one more piece of access-control configuration to keep correct and to explain in a writeup.

## Future re-evaluation triggers

Revisit this ADR if:
- More than one external host needs to reach the cluster (or vice versa), at which point Cloudflare Tunnel's centralized policy management may be worth the added control-plane dependency.
- GPU interruption/reclaim frequency becomes high enough to block `#4` benchmark runs — consider a non-spot instance type or a second provider as a fallback.
- The project moves beyond a single-person tailnet (e.g. a second contributor or a second GPU host), where Tailscale ACL complexity grows non-trivially.

## Final rationale

Tailscale over a public endpoint or a second control-plane (Cloudflare) is the right fit for a two-node private link operated by one person: it keeps `#0`'s scope to exactly what's needed — a GPU host, a private tunnel, and a real embeddings call — without adding infrastructure that isn't earning its keep yet. The model, dimension, and cost fields above are placeholders by design: this ADR is meant to carry the bring-up forward, not to pretend `#0` is already done.
