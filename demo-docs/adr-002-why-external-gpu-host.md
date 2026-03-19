# ADR-002 — Why We Run vLLM on an External GPU Host

- **Status:** Accepted
- **Date:** 2026-03-18
- **Decision owners:** platform / architecture

## Context

The retrieval platform needs a model-serving layer for:

- generating embeddings during ingestion
- generating query embeddings at request time
- optionally generating final answers from retrieved chunks

We chose `vLLM` as the serving engine because it exposes an OpenAI-compatible HTTP interface and is designed for model serving on GPU infrastructure.

The main architecture question was:

**Should GPU inference run inside k3s, or on a separate external GPU host?**

## Decision

We chose to run `vLLM` on an **external GPU host**, outside of the k3s cluster.

For this project, the external GPU host is rented through a low-cost GPU marketplace such as Vast.ai, which markets itself as an affordable GPU cloud / marketplace where GPU instances can be launched quickly.

## Why this fits the project

## 1. Keep k3s simple

The core k3s environment only needs to run:

- `api-go`
- Qdrant
- Grafana
- supporting Kubernetes resources

That is a clean and understandable control plane for a small project.

Running GPUs inside Kubernetes would force the project to take on extra concerns:

- GPU node management
- drivers
- device plugins
- scheduling constraints
- node taints and tolerations
- GPU-specific image/runtime debugging

That is real engineering work, but it is not the main lesson of this project.

## 2. GPU cost profile is different from API cost profile

The API, Qdrant, and Grafana stack can run on modest CPU-backed infrastructure.

The model-serving layer is the expensive part.

Separating it lets us:

- rent GPU only where needed
- experiment with model size independently
- stop, replace, or resize GPU capacity without touching the cluster
- keep the always-on control plane cheaper

That separation is especially useful in a pet project where cost discipline matters.

## 3. Faster iteration on model-serving experiments

An external GPU host makes it easy to test:

- another model
- another quantization strategy
- different `vLLM` startup parameters
- different provider or host class

without turning the whole main cluster into a GPU platform.

This supports the learning goals of the project.

## 4. Clearer architecture story

From a portfolio perspective, this architecture tells a more interesting story:

- lightweight control plane in k3s
- stateful retrieval layer in Qdrant
- isolated inference plane on dedicated GPU hardware

That is a stronger signal than a vague “everything runs somewhere in Kubernetes.”

## Why we did not run GPU in Kubernetes

## Operational complexity

Running GPUs in Kubernetes is absolutely possible, but it would add platform work that is not central here:

- GPU nodes
- node pools
- drivers
- runtime mismatches
- cluster-specific GPU scheduling bugs
- potentially much higher idle cost

For a pet project, that is a lot of complexity for limited architectural gain.

## Idle cost

If the cluster must keep an expensive GPU node attached just to preserve inference capacity, the baseline cost becomes much harder to justify.

A separate GPU rental model is better aligned with experimentation.

## Higher blast radius

A broken GPU driver or model-serving runtime inside the main cluster can complicate cluster operations more broadly.

Keeping the GPU layer outside narrows the blast radius.

## Why Vast.ai

Vast.ai is not chosen because it is perfect. It is chosen because it matches the economics of an exploratory project.

Reasons:

- low-cost access to GPU instances
- quick provisioning
- wide range of hardware classes
- easier experimentation than locking into a more formal cloud GPU setup

For a project whose goal is to demonstrate **practical cost-aware engineering**, this is a meaningful choice. Vast.ai explicitly positions itself as an affordable GPU marketplace with flexible launch options.

## Trade-offs accepted

This decision comes with real downsides.

## 1. Extra network hop

`api-go` must call `vLLM` over the network instead of through local cluster service discovery.

Impact:

- extra latency
- more timeout tuning
- more DNS or routing failure modes

## 2. Separate failure domain

The GPU host can fail independently of k3s.

Possible failure cases:

- provider host restart
- spot/interruption-like reclaim behavior depending on rental type
- model process crash
- SSH access problems
- firewall or TLS misconfiguration

## 3. Separate observability surface

Metrics, logs, and uptime on the GPU side are not automatically visible in cluster dashboards unless wired in deliberately.

## 4. Security and secret handling become more explicit

Because the dependency is external, we must think more carefully about:

- endpoint auth
- TLS or private networking
- egress controls
- secret distribution

## Alternatives considered

## Option A — GPU nodes inside k3s

Rejected for now.

Why:

- too much complexity for the project stage
- cost footprint too high
- cluster becomes harder to explain and operate

## Option B — managed embedding API

Rejected for now.

Why:

- weaker infrastructure signal
- less control over cost and hosting story
- project becomes more of a service-integration demo than an owned platform

## Consequences

## Positive

- cheaper baseline control plane
- flexible GPU experimentation
- clearer separation of concerns
- stronger story around cost-aware architecture

## Negative

- dependency on external network path
- more single points of failure
- harder end-to-end observability without deliberate instrumentation

## Future re-evaluation triggers

Revisit this ADR if:

- request volume justifies always-on dedicated GPU nodes in-cluster
- the project needs stricter latency SLOs
- security requirements demand tighter network locality
- operational pain from the external dependency becomes too high

## Final rationale

We run `vLLM` on an external GPU host because it is the best fit for the current stage of the project:

- cheaper
- simpler for the core cluster
- easier to iterate on
- more honest about trade-offs
- stronger from an architecture-signaling perspective

The cost savings are real, but the bigger value is architectural clarity.
