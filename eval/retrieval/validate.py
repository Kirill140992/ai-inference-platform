#!/usr/bin/env python3
"""Validate the retrieval eval dataset.

Checks every line of retrieval-eval.jsonl:
- parses as JSON with the required fields
- ids are unique
- expected_doc resolves to an existing file in demo-docs/
- answer_snippet is a verbatim substring of that document

Run from the repo root: python3 eval/retrieval/validate.py
"""

import json
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
DATASET = Path(__file__).resolve().parent / "retrieval-eval.jsonl"
DEMO_DOCS_DIR = REPO_ROOT / "demo-docs"

REQUIRED_FIELDS = ("id", "query", "expected_doc", "answer_snippet", "type")


def doc_path(doc_id: str) -> Path:
    return DEMO_DOCS_DIR / f"{doc_id}.md"


def main() -> int:
    errors = []
    seen_ids = set()
    count = 0

    for line_no, line in enumerate(DATASET.read_text().splitlines(), start=1):
        if not line.strip():
            continue
        count += 1

        try:
            pair = json.loads(line)
        except json.JSONDecodeError as exc:
            errors.append(f"line {line_no}: invalid JSON: {exc}")
            continue

        missing = [f for f in REQUIRED_FIELDS if not pair.get(f)]
        if missing:
            errors.append(f"line {line_no}: missing fields: {', '.join(missing)}")
            continue

        pair_id = pair["id"]
        if pair_id in seen_ids:
            errors.append(f"line {line_no}: duplicate id {pair_id!r}")
        seen_ids.add(pair_id)

        path = doc_path(pair["expected_doc"])
        if not path.is_file():
            errors.append(f"{pair_id}: expected_doc {pair['expected_doc']!r} not found at {path}")
            continue

        if pair["answer_snippet"] not in path.read_text():
            errors.append(f"{pair_id}: answer_snippet is not a verbatim substring of {path.name}")

    if errors:
        for err in errors:
            print(f"FAIL: {err}", file=sys.stderr)
        return 1

    print(f"OK: {count} pairs validated against {DEMO_DOCS_DIR}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
