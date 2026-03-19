# ADR-001 — Why We Chose Qdrant

- **Status:** Accepted
- **Date:** 2026-03-18
- **Decision owners:** platform / architecture

## Context

The project needs a vector store for a retrieval platform with the following properties:

- easy local or self-hosted deployment
- good fit for a small self-managed stack
- straightforward API for upsert and search
- support for payload metadata alongside vectors
- visibility into operations and storage
- enough performance for a real demo, without premature over-engineering

At this stage, the goal is not “best possible enterprise vector platform.” The goal is **a credible engineering choice for a self-hosted pet project**.

## Decision

We chose **Qdrant** as the vector database.

Qdrant is an open-source vector search engine designed for vector similarity search with collections, points, payloads, and operational features such as monitoring endpoints and snapshots.

## Why Qdrant fits this project

## 1. Self-hosted by default

This project wants to show infrastructure ownership, not just API consumption.

Qdrant can be self-hosted directly and fits a small Kubernetes-based platform well. That makes it a strong choice for demonstrating:

- stateful operations
- persistent storage thinking
- backup and restore design
- observability of the vector layer

## 2. Purpose-built for vector retrieval

Qdrant is built around vector collections and payload-aware search rather than treating vectors as an add-on.

That matters because the retrieval platform wants first-class support for:

- vector search
- payload metadata
- operationally clear collection model
- direct API interaction for debugging and demos

## 3. Good fit for a learning-focused architecture

This project is intentionally small. Qdrant lets us keep the architecture understandable:

- one stateful service
- one persistent volume
- straightforward data model
- obvious backup target

That simplicity is valuable for a public portfolio project.

## 4. Practical operational features

Qdrant exposes Prometheus/OpenMetrics-style metrics and health endpoints, which fits the observability story well. It also supports collection snapshots for backup and restore.

These features matter because the project is trying to show not just “it runs,” but:

- how it is monitored
- how it is recovered
- where its failure boundaries are

## Alternatives considered

## Option A — Pinecone

Pinecone is a strong managed vector database option and today positions itself as a fully managed, serverless vector database with effortless scaling and production-oriented reliability features.

### Why we did not choose Pinecone

Pinecone is attractive when the goal is:

- fast managed setup
- less infrastructure ownership
- easier scaling without self-managing storage or nodes

But those are not the main goals of this project.

Reasons we did not choose it:

- the project wants to demonstrate self-hosted operational thinking
- using a managed vector database would reduce the visible infrastructure story
- backup, storage, and failure analysis would be less hands-on
- it would make the system look more like “assembled SaaS components” than an owned platform

This is not a criticism of Pinecone. It is simply the wrong signal for this particular project.

## Option B — pgvector

pgvector is a PostgreSQL extension for vector similarity search and works well when vectors need to live close to the rest of relational data. It supports storing vectors inside Postgres and benefits from familiar database workflows, ACID behavior, and joins.

### Why we did not choose pgvector

pgvector is reasonable when:

- Postgres already exists as a strong system-of-record
- vectors are one part of a broader relational application
- the team prefers one database operationally

We did not choose it because:

- this project does not have a strong relational core that naturally centers on Postgres
- the main workload is retrieval, not mixed relational-plus-vector workload
- a purpose-built vector store makes the architecture easier to explain
- operationally, we wanted the vector layer to be explicit rather than buried inside a general-purpose database

For this project, pgvector would have been defensible, but less communicative.

## Trade-offs accepted

By choosing Qdrant, we accept:

- we must operate a dedicated stateful vector service
- we must manage PVCs, backups, and restore drills
- single-node failure is our responsibility unless we add replicas
- there is more infrastructure surface area than with a managed service

These are acceptable trade-offs because they strengthen the engineering story of the project.

## Consequences

## Positive

- clear vector-specific architecture
- stronger self-hosting and SRE signal
- straightforward narrative around stateful recovery
- good fit for public architecture docs and incident writeups

## Negative

- more infrastructure to manage than Pinecone
- more operational burden than pgvector in an already-Postgres-heavy system
- one more stateful service for a small project

## Rejected arguments

These arguments were considered weak and were rejected:

### “Use Pinecone because it is more production-looking”

For this project, a managed service does not automatically send a stronger signal. It may actually hide engineering depth.

### “Use pgvector because one database is always simpler”

One database is not always simpler if it makes the main workload less legible. Clarity matters.

## Future re-evaluation triggers

Revisit this ADR if any of the following become true:

- the project evolves into a strongly relational product
- managed-service speed becomes more important than infra ownership
- dataset scale or SLOs outgrow the current single-node Qdrant design
- the cost/ops ratio changes enough to justify a different architecture

## Final rationale

We chose Qdrant because it strikes the right balance for this project:

- self-hosted
- vector-first
- operationally visible
- small enough to understand
- serious enough to discuss stateful reliability

That is exactly the signal this pet project needs to send.
