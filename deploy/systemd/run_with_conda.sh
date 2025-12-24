#!/usr/bin/env bash
set -euo pipefail

# Run a command after activating a conda env when possible.
# This helps keep systemd units host-agnostic (miniconda vs miniforge paths differ).
#
# Env:
#   AI_THINGS_CONDA_ENV=speech
#   AI_THINGS_CONDA_SH=/opt/miniconda3/etc/profile.d/conda.sh   (optional override)
#
# Usage:
#   run_with_conda.sh <cmd> [args...]

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <cmd> [args...]" >&2
  exit 2
fi

CONDA_ENV="${AI_THINGS_CONDA_ENV:-speech}"
CONDA_SH="${AI_THINGS_CONDA_SH:-}"

find_conda_sh() {
  local candidates=(
    "${CONDA_SH}"
    "/opt/miniconda3/etc/profile.d/conda.sh"
    "/opt/conda/etc/profile.d/conda.sh"
    "/home/grimlock/miniforge3/etc/profile.d/conda.sh"
    "/home/grimlock/miniconda3/etc/profile.d/conda.sh"
    "/usr/etc/profile.d/conda.sh"
  )
  local c
  for c in "${candidates[@]}"; do
    [[ -n "${c}" && -f "${c}" ]] && { echo "${c}"; return 0; }
  done
  return 1
}

if conda_sh="$(find_conda_sh)"; then
  # shellcheck disable=SC1090
  source "${conda_sh}"
  if command -v conda >/dev/null 2>&1; then
    conda activate "${CONDA_ENV}" >/dev/null 2>&1 || true
  fi
else
  echo "WARN: conda.sh not found; running without conda env ${CONDA_ENV}" >&2
fi

exec "$@"


