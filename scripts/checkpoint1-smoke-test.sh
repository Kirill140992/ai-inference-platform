#!/usr/bin/env bash
set -euo pipefail

# Verifies Checkpoint 1 of #0 (docs/book-issues/00-bringup-vast-tailscale.md):
# the Vast.ai GPU host is reachable over Tailscale and vLLM responds --
# with zero involvement from api-go. Run this from the local k3s node.
# A failure here is a network/GPU problem, not a code problem; don't start
# on Checkpoint 2 (wiring api-go) until this passes.
#
# Usage:
#   scripts/checkpoint1-smoke-test.sh <tailscale-host-or-ip> [vllm-port]
#   TAILSCALE_HOST=<host> VLLM_PORT=<port> scripts/checkpoint1-smoke-test.sh
#
# Example:
#   scripts/checkpoint1-smoke-test.sh vast-gpu 8000

TAILSCALE_HOST="${1:-${TAILSCALE_HOST:-}}"
VLLM_PORT="${2:-${VLLM_PORT:-8000}}"

if [[ -z "$TAILSCALE_HOST" ]]; then
  echo "usage: $0 <tailscale-host-or-ip> [vllm-port]" >&2
  echo "   or: TAILSCALE_HOST=<host> VLLM_PORT=<port> $0" >&2
  exit 2
fi

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

echo "== Checkpoint 1 smoke test: GPU + Tailscale reachable (no api-go involved) =="
echo ""

command -v tailscale >/dev/null 2>&1 || fail "tailscale CLI not found on this host — install it before running Checkpoint 1"

echo "-- tailscale status --"
tailscale status || fail "tailscale is not running / not logged in (run: tailscale up)"
echo ""

echo "-- tailscale ping ${TAILSCALE_HOST} --"
tailscale ping -c 1 "$TAILSCALE_HOST" \
  || fail "cannot reach ${TAILSCALE_HOST} over the tailnet (check ACLs, check the GPU host joined the tailnet)"
echo ""

echo "-- curl http://${TAILSCALE_HOST}:${VLLM_PORT}/v1/models --"
response="$(curl --fail --silent --show-error --max-time 10 "http://${TAILSCALE_HOST}:${VLLM_PORT}/v1/models")" \
  || fail "vLLM did not respond on ${TAILSCALE_HOST}:${VLLM_PORT} (is vLLM running? bound to the tailscale interface, not just localhost?)"

echo "$response"
echo ""

model_count="?"
if command -v jq >/dev/null 2>&1; then
  model_count="$(echo "$response" | jq '.data | length' 2>/dev/null || echo "?")"
fi

echo "PASS: Checkpoint 1 is green -- ${TAILSCALE_HOST}:${VLLM_PORT} is reachable over Tailscale and vLLM returned a model list (models reported: ${model_count})."
echo "Next: record the exact model name + embedding dimension in demo-docs/adr-003-vast-tailscale-bringup.md, then move to Checkpoint 2 (wire api-go)."
