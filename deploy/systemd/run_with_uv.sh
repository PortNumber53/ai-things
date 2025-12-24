#!/usr/bin/env bash
set -euo pipefail

# Run a command with a configured virtualenv on PATH.
# This replaces the previous conda-based wrapper.
#
# Configure via /etc/ai-things/systemd.env (loaded by systemd units):
#   AI_THINGS_VENV=/deploy/ai-things/venvs/runtime
#
# Usage:
#   run_with_uv.sh <cmd> [args...]

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <cmd> [args...]" >&2
  exit 2
fi

VENV_DIR="${AI_THINGS_VENV:-}"
if [[ -n "${VENV_DIR}" ]]; then
  if [[ -x "${VENV_DIR}/bin/python" ]]; then
    export VIRTUAL_ENV="${VENV_DIR}"
    export PATH="${VENV_DIR}/bin:${PATH}"
  else
    echo "WARN: AI_THINGS_VENV is set but missing python: ${VENV_DIR}/bin/python" >&2
  fi
fi

exec "$@"


