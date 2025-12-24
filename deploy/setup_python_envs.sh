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
#   AI_THINGS_UV_PYTHON=3.11       (preferred Python version for venvs; avoids 3.13 wheel gaps)
#   AI_THINGS_PYTHON_PROJECTS=all  (or comma-separated list like "utility,podcast")
#   AI_THINGS_PYTHON_SETUP_HEAVY=1 (include heavy projects like tortoise-tts/ollama)
#   AI_THINGS_PYTHON_STRICT=1      (fail deployment on first error)

DEPLOY_BASE_PATH="${1:-/deploy/ai-things}"
CURRENT_PATH="${2:-${DEPLOY_BASE_PATH%/}/current}"
UV_PYTHON="${AI_THINGS_UV_PYTHON:-3.11}"
STRICT="${AI_THINGS_PYTHON_STRICT:-0}"
PROJECTS_RAW="${AI_THINGS_PYTHON_PROJECTS:-}"
SETUP_HEAVY="${AI_THINGS_PYTHON_SETUP_HEAVY:-0}"

VENV_ROOT="${DEPLOY_BASE_PATH%/}/venvs"
mkdir -p "${VENV_ROOT}"

echo "python-env: base=${DEPLOY_BASE_PATH} current=${CURRENT_PATH} venv_root=${VENV_ROOT} uv_python=${UV_PYTHON} strict=${STRICT}"

if ! command -v uv >/dev/null 2>&1; then
  echo "python-env: ERROR: uv not found on PATH" >&2
  if [[ "${STRICT}" == "1" ]]; then
    exit 127
  fi
  exit 0
fi

ensure_venv() {
  local venv_path="$1"
  if [[ ! -x "${venv_path}/bin/python" ]]; then
    echo "python-env: create venv ${venv_path} (python=${UV_PYTHON})"
    uv venv --allow-existing -p "${UV_PYTHON}" "${venv_path}"
  fi
}

install_reqs() {
  local venv_path="$1"
  local req_file="$2"
  uv pip install -p "${venv_path}/bin/python" -r "${req_file}"
}

echo "python-env: ensure runtime venv"
runtime_venv="${VENV_ROOT}/runtime"
runtime_req="${CURRENT_PATH}/deploy/requirements-runtime.txt"
if [[ -f "${runtime_req}" ]]; then
  ensure_venv "${runtime_venv}"
  set +e
  install_reqs "${runtime_venv}" "${runtime_req}"
  rc=$?
  set -e
  if [[ $rc -ne 0 ]]; then
    echo "python-env: ERROR installing runtime requirements (exit=${rc})"
    if [[ "${STRICT}" == "1" ]]; then
      exit $rc
    fi
  fi
else
  echo "python-env: skip runtime venv (missing ${runtime_req})"
fi

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
  ensure_venv "${venv}"

  echo "python-env: install ${proj} requirements"
  set +e
  install_reqs "${venv}" "${req}"
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


