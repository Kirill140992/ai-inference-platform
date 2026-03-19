# Qdrant Backup and Restore Runbook

## Purpose

This runbook describes what must be backed up for Qdrant, how to verify storage, how to create and restore backups, and what risks remain in a single-node deployment.

In this platform, Qdrant is the primary stateful component. Treat it accordingly.

## Why this matters

Without Qdrant, the platform loses:

- document embeddings
- chunk payloads
- collection configuration
- the live retrieval index

The rest of the platform can be recreated from Git and container images. Qdrant data cannot.

## What exactly must be backed up

For this project, the minimum backup scope is:

- Qdrant collection data
- Qdrant collection configuration
- payload data stored with vectors
- index structures needed for fast recovery
- PVC-level storage or Qdrant snapshots
- optionally exported manifest and config references for disaster recovery context

If the platform stores raw chunk text inside Qdrant payload, then Qdrant contains both retrieval vectors and the searchable text context. That makes the backup even more critical.

## Where Qdrant data lives

### Inside the container

Qdrant typically uses the `/qdrant/storage` path inside the container for persistent data.

### On the host or cluster side

In Kubernetes, that directory is backed by a **PersistentVolumeClaim** mounted into the Qdrant pod.

In a local Docker quickstart, Qdrant commonly persists under `./qdrant_storage`. In Kubernetes, the practical storage location is the PVC and its backing disk.

## Verify the PVC before touching backups

Check the StatefulSet and PVC:

```bash
kubectl -n ai-platform get statefulset qdrant
kubectl -n ai-platform get pvc
kubectl -n ai-platform describe pvc <qdrant-pvc-name>
```

Check the pod and mount:

```bash
kubectl -n ai-platform get pods -l app=qdrant -o wide
kubectl -n ai-platform exec -it statefulset/qdrant -- sh
ls -lah /qdrant/storage
df -h /qdrant/storage
```

You want to confirm:

- the PVC is `Bound`
- the mount exists
- the filesystem has enough free space
- Qdrant is actually writing to the mounted path, not to ephemeral container storage

## Backup strategies

There are two valid layers for backup in this project.

## Option A — Qdrant snapshot backup

Preferred for logical recovery of collections.

Qdrant supports **snapshots**, which are tar archives containing collection data and configuration and are intended for efficient export and restore.

Advantages:

- portable
- collection-aware
- faster recovery than rebuilding embeddings from scratch
- does not require understanding the underlying disk format

Disadvantages:

- still depends on Qdrant being functional enough to create snapshots
- must be copied out of the pod or downloaded after creation

### Create a snapshot

Example API call from a temporary port-forward or reachable service endpoint:

```bash
kubectl -n ai-platform port-forward svc/qdrant 6333:6333
```

In another terminal:

```bash
curl -X POST http://127.0.0.1:6333/collections/knowledge-base/snapshots
```

List snapshots:

```bash
curl http://127.0.0.1:6333/collections/knowledge-base/snapshots
```

Download a snapshot:

```bash
curl -o knowledge-base.snapshot \
  http://127.0.0.1:6333/collections/knowledge-base/snapshots/<snapshot-name>
```

Store the snapshot outside the cluster:

- object storage
- encrypted backup directory
- separate host from the cluster itself

## Option B — PVC or volume-level backup

Preferred when you want a lower-level disaster recovery copy.

Advantages:

- captures the entire mounted storage
- useful when Qdrant API is not healthy enough for snapshot creation

Disadvantages:

- more coupled to the storage backend
- rougher restore path
- greater risk of inconsistent copies if taken at the wrong time

For a small pet project, logical snapshots plus occasional PVC-level copies are enough.

## Backup frequency

Recommended baseline:

- snapshot before schema or model changes
- snapshot before large ingestion runs
- daily snapshot if the dataset changes often
- weekly snapshot at minimum for a stable demo environment

Also keep at least one known-good backup taken after a successful ingestion and validation cycle.

## Backup naming convention

Use a predictable format:

```text
qdrant-knowledge-base-YYYYMMDD-HHMM.snapshot
```

Example:

```text
qdrant-knowledge-base-20260318-1030.snapshot
```

This makes incident handling much less painful.

## Restore scenarios

## Scenario 1 — Pod was deleted but PVC still exists

This is the easiest case.

Action:

- let the StatefulSet recreate the pod
- confirm the same PVC reattaches
- verify Qdrant starts correctly
- check collection availability

Commands:

```bash
kubectl -n ai-platform get pods -l app=qdrant
kubectl -n ai-platform get pvc
kubectl -n ai-platform logs statefulset/qdrant --tail=200
curl -fsS http://<QDRANT_URL>:6333/collections
```

If the PVC is intact, restore may not be needed.

## Scenario 2 — PVC is intact but collection is corrupted or missing

Use a Qdrant snapshot restore.

Port-forward or expose the service, then upload the snapshot using the restore API or Web UI.

General flow:

1. ensure target collection name is known
2. upload snapshot
3. trigger restore
4. verify collection count and point count
5. run a retrieval smoke test

After restore, validate:

```bash
curl http://127.0.0.1:6333/collections
curl http://127.0.0.1:6333/collections/knowledge-base
```

## Scenario 3 — PVC lost entirely

This is the true disaster case in a single-node setup.

Recovery path:

1. recreate the PVC and Qdrant pod
2. restore from off-cluster snapshot
3. verify collection integrity
4. run ingestion only for any missing delta since the last snapshot

If there is no backup, the only option is full re-ingestion from the original source documents.

## Restore checklist

After any restore, confirm all of the following:

- Qdrant pod is Running and Ready
- expected collection exists
- collection point count looks sane
- sample payload fields are present
- retrieval query returns expected results
- API `/ready` is green
- ingestion does not produce dimension mismatch errors

## Storage validation commands

These commands are useful before and after restore:

```bash
kubectl -n ai-platform get pvc
kubectl -n ai-platform describe pvc <qdrant-pvc-name>
kubectl -n ai-platform exec -it statefulset/qdrant -- df -h /qdrant/storage
kubectl -n ai-platform exec -it statefulset/qdrant -- ls -lah /qdrant/storage
kubectl -n ai-platform logs statefulset/qdrant --tail=200
```

## Risks in a single-node setup

This project accepts several risks by design.

### No replica redundancy

A single Qdrant node means there is no in-cluster failover.

### PVC is a hard dependency

If the underlying disk fails and backups do not exist, indexed data is gone.

### Backup gaps can create ingestion rebuild cost

If embeddings must be regenerated from source documents, recovery depends on the external `vLLM` GPU path being healthy and may take a long time.

### Model drift risk

If you rebuild without pinning the same embedding model, restored vectors may not be comparable to older data.

### Hidden corruption risk

A pod may restart successfully while the collection quality is still wrong. Recovery must always include a real retrieval smoke test, not just a pod health check.

## What not to do

Avoid these mistakes:

- assuming pod recreation equals data safety
- backing up only manifests and not Qdrant data
- storing backups on the same disk as the primary PVC
- changing embedding model during restore without a reindex plan
- declaring recovery complete before querying the collection

## Suggested next steps

For a stronger platform story, add:

- scheduled snapshot job
- retention policy for snapshots
- off-cluster encrypted backup target
- documented recovery time objective
- periodic restore drill
