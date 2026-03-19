#!/usr/bin/env python3

import json
import re
import sys
from pathlib import Path
from urllib import request, error

API_URL = "http://127.0.0.1:8000/documents"
DEMO_DOCS_DIR = Path("demo-docs")
CHUNK_SIZE = 800
CHUNK_OVERLAP = 120
DRY_RUN = True


def slug_to_title(slug: str) -> str:
    parts = slug.replace("_", "-").split("-")
    return " ".join(word.capitalize() for word in parts if word)


def build_document_id(file_path: Path) -> str:
    return file_path.stem.strip().lower().replace("_", "-").replace(" ", "-")


def post_document(payload: dict) -> dict:
    body = json.dumps(payload).encode("utf-8")

    req = request.Request(
        API_URL,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except error.HTTPError as exc:
        response_body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(
            f"HTTP {exc.code} from API: {response_body}"
        ) from exc
    except error.URLError as exc:
        raise RuntimeError(
            f"Could not reach API at {API_URL}: {exc}"
        ) from exc


def main() -> int:
    if not DEMO_DOCS_DIR.exists():
        print(f"Directory not found: {DEMO_DOCS_DIR}", file=sys.stderr)
        return 1

    files = sorted(DEMO_DOCS_DIR.glob("*.md"))
    if not files:
        print(f"No markdown files found in: {DEMO_DOCS_DIR}", file=sys.stderr)
        return 1

    total_files = 0
    total_chunks = 0
    failed = 0

    print(f"Using API endpoint: {API_URL}")
    print(f"Reading documents from: {DEMO_DOCS_DIR}")
    print("")

    for file_path in files:
        total_files += 1

        content = file_path.read_text(encoding="utf-8").strip()
        if not content:
            print(f"[SKIP] {file_path.name}: empty file")
            continue

        document_id = build_document_id(file_path)
        title = slug_to_title(file_path.stem)

        payload = {
            "document_id": document_id,
            "title": title,
            "source": f"demo-docs/{file_path.name}",
            "content": content,
            "chunk_size": CHUNK_SIZE,
            "chunk_overlap": CHUNK_OVERLAP,
            "dry_run": DRY_RUN,
        }

        try:
            result = post_document(payload)
        except Exception as exc:
            failed += 1
            print(f"[FAIL] {file_path.name}: {exc}")
            continue

        chunk_count = result.get("chunk_count", 0)
        chunking_version = result.get("chunking_version", "unknown")
        chunking_mode = result.get("chunking_mode", "unknown")

        total_chunks += chunk_count

        print(
            f"[OK] {file_path.name} | "
            f"document_id={document_id} | "
            f"chunks={chunk_count} | "
            f"mode={chunking_mode} | "
            f"version={chunking_version}"
        )

        chunks = result.get("chunks", [])
        for chunk in chunks[:2]:
            preview = chunk.get("text", "").replace("\n", " ").strip()
            preview = re.sub(r"\s+", " ", preview)
            preview = preview[:120]
            print(
                f"      chunk_index={chunk.get('chunk_index')} "
                f"chunk_id={chunk.get('chunk_id')} "
                f"preview={preview}"
            )

        if len(chunks) > 2:
            print(f"      ... {len(chunks) - 2} more chunks")

    print("")
    print("Summary")
    print(f"  files processed: {total_files}")
    print(f"  total chunks:    {total_chunks}")
    print(f"  failed:          {failed}")

    return 0 if failed == 0 else 2


if __name__ == "__main__":
    raise SystemExit(main())