#!/usr/bin/env python3
"""Offline retrieval eval harness (#3, docs/book-issues/03-offline-eval.md).

Runs every query in retrieval-eval.jsonl through a retriever, scores the
ranked results against the labeled expected document, and prints a metrics
table (recall@k / MRR / nDCG@k, overall and per query type).

Retrievers:
  --retriever lexical   Token-overlap baseline over demo-docs/*.md.
                        Deliberately dumb: it's the "before" number that
                        dense retrieval (#0) and the reranker (#1) must
                        beat. Works today, no services needed.
  --retriever api       Calls the platform's search endpoint (lands with
                        #0 Checkpoint 2). Contract expected from api-go:
                          POST {api-url}/search
                          body:     {"query": "...", "top_k": N}
                          response: {"results": [{"document_id": "...", ...}, ...]}
                        Results must be ordered best-first; the harness
                        dedupes chunk hits into a doc-level ranking.

Usage:
  python3 eval/retrieval/harness.py --retriever lexical
  python3 eval/retrieval/harness.py --retriever api --api-url http://localhost:8000
  python3 eval/retrieval/harness.py --retriever lexical --save lexical-baseline

NOTE: metrics.py is not implemented yet (that's the #3 learning work) —
until then this exits with a pointer to test_metrics.py.
"""

import argparse
import json
import re
import subprocess
import sys
import urllib.request
from collections import defaultdict
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import metrics  # noqa: E402

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_DATASET = Path(__file__).resolve().parent / "retrieval-eval.jsonl"
DEFAULT_DOCS_DIR = REPO_ROOT / "demo-docs"
BASELINES_DIR = Path(__file__).resolve().parent / "baselines"


def load_dataset(path: Path) -> list:
    pairs = []
    for line in path.read_text().splitlines():
        if line.strip():
            pairs.append(json.loads(line))
    return pairs


def tokenize(text: str) -> list:
    return [t for t in re.split(r"[^a-z0-9]+", text.lower()) if len(t) >= 3]


class LexicalRetriever:
    """Doc-level token-overlap baseline. No tf-idf, no stemming — on
    purpose. If dense retrieval can't beat this, something is wrong."""

    def __init__(self, docs_dir: Path):
        self.doc_tokens = {}
        for doc in sorted(docs_dir.glob("*.md")):
            self.doc_tokens[doc.stem] = set(tokenize(doc.read_text()))

    def retrieve(self, query: str, top_k: int) -> list:
        q_tokens = set(tokenize(query))
        scored = [
            (len(q_tokens & tokens), doc_id)
            for doc_id, tokens in self.doc_tokens.items()
        ]
        scored.sort(key=lambda pair: (-pair[0], pair[1]))
        return [doc_id for score, doc_id in scored[:top_k] if score > 0]


class ApiRetriever:
    """Calls api-go's search endpoint (the #0 Checkpoint 2 contract)."""

    def __init__(self, api_url: str):
        self.search_url = api_url.rstrip("/") + "/search"

    def retrieve(self, query: str, top_k: int) -> list:
        body = json.dumps({"query": query, "top_k": top_k}).encode()
        req = urllib.request.Request(
            self.search_url, data=body, headers={"Content-Type": "application/json"}
        )
        with urllib.request.urlopen(req, timeout=30) as resp:
            payload = json.load(resp)

        ranked_docs = []
        for result in payload["results"]:
            doc_id = result["document_id"]
            if doc_id not in ranked_docs:  # chunk hits → doc-level ranking
                ranked_docs.append(doc_id)
        return ranked_docs[:top_k]


def evaluate(pairs: list, retriever, ks: list, top_k: int) -> dict:
    """Returns {"overall": {...}, "by_type": {type: {...}}, "misses": [...]}."""
    per_query = []
    misses = []

    for pair in pairs:
        ranked = retriever.retrieve(pair["query"], top_k)
        relevant = {pair["expected_doc"]}

        row = {"type": pair["type"], "mrr": metrics.mrr(relevant, ranked)}
        for k in ks:
            row[f"recall@{k}"] = metrics.recall_at_k(relevant, ranked, k)
            row[f"ndcg@{k}"] = metrics.ndcg_at_k(relevant, ranked, k)
        per_query.append(row)

        if row[f"recall@{max(ks)}"] == 0.0:
            misses.append({"id": pair["id"], "query": pair["query"], "expected": pair["expected_doc"], "got": ranked[:3]})

    def aggregate(rows):
        agg = {}
        for key in rows[0]:
            if key == "type":
                continue
            agg[key] = sum(r[key] for r in rows) / len(rows)
        agg["n"] = len(rows)
        return agg

    by_type = defaultdict(list)
    for row in per_query:
        by_type[row["type"]].append(row)

    return {
        "overall": aggregate(per_query),
        "by_type": {t: aggregate(rows) for t, rows in sorted(by_type.items())},
        "misses": misses,
    }


def print_report(report: dict, ks: list) -> None:
    metric_cols = ["mrr"] + [f"recall@{k}" for k in ks] + [f"ndcg@{k}" for k in ks]
    header = f"{'segment':<24} {'n':>3} " + " ".join(f"{m:>9}" for m in metric_cols)
    print(header)
    print("-" * len(header))

    def line(name, agg):
        print(f"{name:<24} {agg['n']:>3} " + " ".join(f"{agg[m]:>9.3f}" for m in metric_cols))

    line("OVERALL", report["overall"])
    for type_name, agg in report["by_type"].items():
        line(type_name, agg)

    if report["misses"]:
        print(f"\ncomplete misses (expected doc not in top-{max(ks)}):")
        for miss in report["misses"]:
            print(f"  {miss['id']}: expected {miss['expected']!r}, got {miss['got']} — {miss['query'][:70]}")


def save_baseline(report: dict, label: str, args: argparse.Namespace) -> Path:
    BASELINES_DIR.mkdir(exist_ok=True)
    commit = subprocess.run(
        ["git", "rev-parse", "--short", "HEAD"], capture_output=True, text=True, cwd=REPO_ROOT
    ).stdout.strip()
    out = {
        "label": label,
        "commit": commit,
        "retriever": args.retriever,
        "top_k": args.top_k,
        "overall": report["overall"],
        "by_type": report["by_type"],
    }
    path = BASELINES_DIR / f"{label}.json"
    path.write_text(json.dumps(out, indent=2) + "\n")
    return path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--dataset", type=Path, default=DEFAULT_DATASET)
    parser.add_argument("--docs-dir", type=Path, default=DEFAULT_DOCS_DIR)
    parser.add_argument("--retriever", choices=["lexical", "api"], default="lexical")
    parser.add_argument("--api-url", default="http://localhost:8000")
    parser.add_argument("--top-k", type=int, default=10, help="how many docs to retrieve per query")
    parser.add_argument("--ks", default="1,3,5,10", help="comma-separated cutoffs for recall@k / ndcg@k")
    parser.add_argument("--save", metavar="LABEL", help="write results to baselines/LABEL.json for before/after comparisons")
    args = parser.parse_args()

    ks = sorted(int(k) for k in args.ks.split(","))
    pairs = load_dataset(args.dataset)

    if args.retriever == "lexical":
        retriever = LexicalRetriever(args.docs_dir)
    else:
        retriever = ApiRetriever(args.api_url)

    try:
        report = evaluate(pairs, retriever, ks, args.top_k)
    except NotImplementedError as exc:
        print(f"metrics are not implemented yet: {exc}", file=sys.stderr)
        print("implement eval/retrieval/metrics.py until eval/retrieval/test_metrics.py passes — that's the #3 work.", file=sys.stderr)
        return 2

    print(f"dataset: {args.dataset.name} ({len(pairs)} pairs) | retriever: {args.retriever} | top_k: {args.top_k}\n")
    print_report(report, ks)

    if args.save:
        path = save_baseline(report, args.save, args)
        print(f"\nbaseline saved: {path.relative_to(REPO_ROOT)}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
