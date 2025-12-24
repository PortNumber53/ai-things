#!/usr/bin/env bash
set -euo pipefail

# Idempotently set up Python virtualenvs for the Python sub-projects shipped in this repo.
# We keep venvs under $DEPLOY_BASE_PATH/venvs so they persist across releases and don't
# require a full reinstall on every deploy.
#
# Usage:
#   ./deploy/setup_python_envs.sh /deploy/ai-things /deploy/ai-things/current
#
# Env:
#   PYTHON=python3  (override python executable used to create venvs)

DEPLOY_BASE_PATH="${1:-/deploy/ai-things}"
CURRENT_PATH="${2:-${DEPLOY_BASE_PATH%/}/current}"
PYTHON_BIN="${PYTHON:-python3}"

VENV_ROOT="${DEPLOY_BASE_PATH%/}/venvs"
mkdir -p "${VENV_ROOT}"

echo "python-env: base=${DEPLOY_BASE_PATH} current=${CURRENT_PATH} venv_root=${VENV_ROOT}"

projects=(
  "auto-subtitles-generator"
  "podcast"
  "utility"
  "tortoise-tts"
  "ollama"
  "metavoice-src"
  "OpenVoice"
)

for proj in "${projects[@]}"; do
  req="${CURRENT_PATH}/${proj}/requirements.txt"
  if [[ ! -f "${req}" ]]; then
    echo "python-env: skip ${proj} (no requirements.txt)"
    continue
  fi

  venv="${VENV_ROOT}/${proj}"
  if [[ ! -x "${venv}/bin/python" ]]; then
    echo "python-env: create venv ${venv}"
    "${PYTHON_BIN}" -m venv "${venv}"
  fi

  echo "python-env: install ${proj} requirements"
  "${venv}/bin/python" -m pip install --upgrade pip setuptools wheel
  "${venv}/bin/python" -m pip install --no-cache-dir -r "${req}"
done

echo "python-env: done"


