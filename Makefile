# ai-inference-platform — developer entrypoints.
# Targets mirror the CLAUDE.md definition-of-done checks; CI runs the same
# commands in .github/workflows/security-pipeline.yaml (unit-tests job).

.PHONY: help test vet fmt-check eval mock-vllm bench

help:
	@echo "Targets:"
	@echo "  test       - go test ./... (app/api-go)"
	@echo "  vet        - go vet ./... (app/api-go)"
	@echo "  fmt-check  - fail if any Go file is not gofmt'd"
	@echo "  eval       - validate the retrieval eval dataset (harness itself is #3, not built yet)"
	@echo "  mock-vllm  - run the local OpenAI-compatible vLLM stub on :8001"
	@echo "  bench      - inference benchmark (#4, not built yet)"

test:
	cd app/api-go && go test ./...

vet:
	cd app/api-go && go vet ./...

fmt-check:
	@unformatted="$$(cd app/api-go && gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt'd:"; echo "$$unformatted"; exit 1; \
	fi

eval:
	python3 eval/retrieval/validate.py

mock-vllm:
	cd app/api-go && go run ./cmd/mock-vllm

bench:
	@echo "not implemented yet — lands with docs/book-issues/04-inference-benchmark.md (needs #0 first)"; exit 1
