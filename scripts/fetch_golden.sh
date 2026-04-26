#!/usr/bin/env bash
set -euo pipefail
# Fetches the kubescape-operator-1.30.5 release body, strips the auto-generated
# ## New Contributors section (GitHub adds it on creation; our tool does not reproduce it),
# and normalizes CRLF -> LF.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOLDEN="$SCRIPT_DIR/../internal/render/testdata/golden_kubescape-operator-1.30.5.md"
gh api repos/kubescape/helm-charts/releases/tags/kubescape-operator-1.30.5 --jq '.body' \
  | tr -d '\r' \
  | awk '/^## New Contributors$/{skip=1} /^\*\*Full Changelog\*\*/{skip=0} !skip{print}' \
  | sed '/^$/N;/^\n$/d' \
  > "$GOLDEN"
echo "Golden file updated: $GOLDEN"
