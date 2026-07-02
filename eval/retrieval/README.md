# Retrieval eval dataset (#3)

Labeled `query → expected document` pairs over the `demo-docs/` corpus. This is the measuring stick for `#3` (offline eval harness) and the before/after gate for `#1` (reranker) and `#2` (hybrid search). Target size: **30–50 pairs** (currently 50 — the target is met; when new docs land in `demo-docs/`, add pairs for them and retire any that stop being answerable).

Curation process for the bulk of the set (q013–q050): candidate pairs were authored per-document, then every candidate was cross-checked against all other corpus docs for ambiguity; ambiguous ones were either dropped or re-sharpened so only the expected document answers them. Queries that merely copied incident details or enumerated the answer options were rejected as too leading.

## Format

One JSON object per line (`retrieval-eval.jsonl`):

```json
{"id": "q001", "query": "...", "expected_doc": "rpc-latency-postmortem", "answer_snippet": "...", "type": "postmortem-lookup"}
```

- `id` — stable identifier (`qNNN`), never reused or renumbered; metrics history must stay comparable across dataset growth.
- `query` — a realistic user question, phrased the way an operator would actually ask it — **not** a copy of the document's title or headings (that would make lexical search look artificially good).
- `expected_doc` — document id of the file that answers the query: the `demo-docs/` filename stem, lowercased, `_`/spaces → `-` (the same convention as `build_document_id` in `scripts/preview_ingest_demo_docs.py`).
- `answer_snippet` — a **verbatim** substring of the expected document containing the answer. This is the chunk-level anchor: chunk ids (`doc::chunk-NNN`) change whenever `chunk_size`/`chunk_overlap` change, so the harness should instead resolve "which chunk contains this snippet" at eval time. Verbatim matters — copy-paste from the doc, don't paraphrase.
- `type` — coarse category (`postmortem-lookup`, `runbook-howto`, `adr-why`, `architecture-lookup`, `security-lookup`) so metrics can later be broken down by query kind.

## Labeling guidelines

- Every pair must be answerable from exactly one primary document. If two docs both answer it, either sharpen the query or skip it.
- Vary the phrasing: some queries mention component names (`Qdrant`, `vLLM`), some describe only symptoms ("documents not showing up in search") — symptom-style queries are the ones that actually separate dense retrieval from keyword matching.
- Don't overfit to one doc: spread pairs across runbooks, ADRs, postmortems, and design docs.
- When a new doc lands in `demo-docs/`, add 1–3 pairs for it in the same change.

## Validation

`python3 eval/retrieval/validate.py` checks that every line parses, every `expected_doc` exists, and every `answer_snippet` is verbatim in its document. Run it after adding pairs.
