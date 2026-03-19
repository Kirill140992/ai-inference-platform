# Demo Docs Pack for the AI Retrieval and Inference Platform

This folder contains a cleaned demo corpus for the project knowledge base.

The documents are written to be safe for public use and consistent with the current project naming:

- namespace: `ai-platform`
- vector collection: `knowledge-base`
- API service: `api-go`
- vector database: `Qdrant`
- external GPU dependency: `vLLM` on a separate GPU host

## Included documents

- architecture-overview.md
- deploy-runbook.md
- qdrant-backup-restore-runbook.md
- incident-sync-lag-postmortem.md
- rpc-latency-postmortem.md
- adr-001-why-qdrant.md
- adr-002-why-external-gpu-host.md
- observability-guide.md
- oncall-checklist.md
- knowledge-ingestion-design.md

## Notes

These files are designed as a public demo corpus for indexing and retrieval tests.
They describe a realistic internal engineering knowledge base without using NDA-protected material.
Some parts describe the current implemented state of the project, while others describe the target design for ingestion, retrieval, and answer generation.
