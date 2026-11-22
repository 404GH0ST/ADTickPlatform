#!/usr/bin/env bash
# Alias for the event-shaped production smoke. smoke-prod-short-match is now the
# 10-team / 180s-tick / 6-challenge fleet smoke; this name is kept for
# discoverability and forwards to it unchanged.
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec "${ROOT_DIR}/scripts/smoke-prod-short-match.sh" "$@"
