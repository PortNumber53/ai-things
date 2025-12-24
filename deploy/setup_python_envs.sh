#!/usr/bin/env bash
set -euo pipefail

# Idempotently set up Python virtualenvs for Python sub-projects shipped in this repo.
# We keep venvs under $DEPLOY_BASE_PATH/venvs so they persist across releases.
#
# IMPORTANT: Some subprojects have heavy deps (Rust, CUDA, etc.) and can fail to build wheels
# depending on host Python version/arch. By default we only install a small "light" set and we
# do NOT fail the deployment if a project env fails. Set AI_THINGS_PYTHON_STRICT=1 to fail fast.
#
# Usage:
#   ./deploy/setup_python_envs.sh /deploy/ai-things /deploy/ai-things/current
#
# Env:
#   PYTHON=python3                 (override python executable used to create venvs)
#   AI_THINGS_PYTHON_PROJECTS=all  (or comma-separated list like "utility,podcast")
#   AI_THINGS_PYTHON_SETUP_HEAVY=1 (include heavy projects like tortoise-tts/ollama)
#   AI_THINGS_PYTHON_STRICT=1      (fail deployment on first error)

DEPLOY_BASE_PATH="${1:-/deploy/ai-things}"
CURRENT_PATH="${2:-${DEPLOY_BASE_PATH%/}/current}"
PYTHON_BIN="${PYTHON:-python3}"
STRICT="${AI_THINGS_PYTHON_STRICT:-0}"
PROJECTS_RAW="${AI_THINGS_PYTHON_PROJECTS:-}"
SETUP_HEAVY="${AI_THINGS_PYTHON_SETUP_HEAVY:-0}"

VENV_ROOT="${DEPLOY_BASE_PATH%/}/venvs"
mkdir -p "${VENV_ROOT}"

PY_VER="$("${PYTHON_BIN}" -c 'import sys; print(".".join(map(str, sys.version_info[:3])))' 2>/dev/null || echo "unknown")"
echo "python-env: base=${DEPLOY_BASE_PATH} current=${CURRENT_PATH} venv_root=${VENV_ROOT} python=${PYTHON_BIN} ver=${PY_VER} strict=${STRICT}"

# Small set that should be relatively safe to install on most hosts.
light_projects=(
  "auto-subtitles-generator"
  "podcast"
  "utility"
)

heavy_projects=(
  "tortoise-tts"
  "ollama"
  "metavoice-src"
  "OpenVoice"
)

projects=()
if [[ "${PROJECTS_RAW}" == "all" ]]; then
  projects=("${light_projects[@]}" "${heavy_projects[@]}")
elif [[ -n "${PROJECTS_RAW}" ]]; then
  IFS=',' read -r -a projects <<<"${PROJECTS_RAW}"
else
  projects=("${light_projects[@]}")
  if [[ "${SETUP_HEAVY}" == "1" ]]; then
    projects+=("${heavy_projects[@]}")
  fi
fi

failures=0
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
  set +e
  "${venv}/bin/python" -m pip install --upgrade pip setuptools wheel
  "${venv}/bin/python" -m pip install --no-cache-dir --prefer-binary -r "${req}"
  rc=$?
  set -e
  if [[ $rc -ne 0 ]]; then
    echo "python-env: ERROR installing ${proj} (exit=${rc})"
    failures=$((failures + 1))
    if [[ "${STRICT}" == "1" ]]; then
      exit $rc
    fi
  fi
done

if [[ $failures -gt 0 ]]; then
  echo "python-env: done (failures=${failures})"
else
  echo "python-env: done"
fi


