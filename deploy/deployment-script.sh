#!/bin/bash

set -euo pipefail
set -x

host="${1:-}"
DEPLOY_BASE_PATH="${2:-}"
DEPLOYMENT_RELEASE_PATH="${3:-}"
DEPLOYMENT_PATH="${4:-}"
TIMESTAMP="${5:-}"

if [[ -z "${host}" || -z "${DEPLOY_BASE_PATH}" || -z "${DEPLOYMENT_RELEASE_PATH}" || -z "${DEPLOYMENT_PATH}" || -z "${TIMESTAMP}" ]]; then
  echo "Usage: $0 <host> <deploy_base_path> <deployment_release_path> <deployment_path> <timestamp>" >&2
  exit 2
fi

hostname
pwd

echo "Deploy host=${host} base=${DEPLOY_BASE_PATH} path=${DEPLOYMENT_PATH} ts=${TIMESTAMP}"

echo "Updating release symlink"
cd "${DEPLOY_BASE_PATH}"
ln -sfn "${DEPLOYMENT_PATH}" ./current

echo "-Preparing Python environments"
bash /deploy/ai-things/current/deploy/setup_python_envs.sh "${DEPLOY_BASE_PATH}" "${DEPLOY_BASE_PATH%/}/current"

echo "-Preparing systemd environment file"
sudo mkdir -p /etc/ai-things
if [[ ! -f /etc/ai-things/systemd.env ]]; then
  sudo tee /etc/ai-things/systemd.env >/dev/null <<EOF
# Deployed by ai-things deploy script. Override locally if needed.
AI_THINGS_VENV=${DEPLOY_BASE_PATH%/}/venvs/runtime
EOF
  sudo chmod 644 /etc/ai-things/systemd.env || true
fi

link_system_service() {
  local src="$1"
  local dest_name="${2:-$(basename "$src")}"
  sudo ln -sfn "$src" "/etc/systemd/system/${dest_name}"
}

link_user_service() {
  local src="$1"
  local dest_dir="${HOME}/.config/systemd/user"
  mkdir -p "${dest_dir}"
  ln -sfn "$src" "${dest_dir}/$(basename "$src")"
}

reload_systemd_system() {
  sudo systemctl daemon-reload
}

reload_systemd_user() {
  systemctl --user daemon-reload
}

enable_now_system() {
  sudo systemctl enable --now "$@"
}

enable_now_user() {
  systemctl --user enable --now "$@"
}

try_restart_system() {
  sudo systemctl try-restart "$@" || true
}

try_restart_user() {
  systemctl --user try-restart "$@" || true
}

  common_service_dir="/deploy/ai-things/current/deploy/systemd"
  host_service_dir="/deploy/ai-things/current/deploy/${host}/systemd"

  echo "-Preparing systemd files (common=${common_service_dir} host=${host_service_dir})"

case "${host}" in
  ideapad5)
    # User-level systemd on this host.
    # Prefer host-specific user unit if present; fall back to common.
    if [[ -f "${host_service_dir}/generate_wav.service" ]]; then
      link_user_service "${host_service_dir}/generate_wav.service"
    elif [[ -f "${common_service_dir}/generate_wav.service" ]]; then
      link_user_service "${common_service_dir}/generate_wav.service"
    fi
    reload_systemd_user
    enable_now_user generate_wav.service
    try_restart_user generate_wav.service
    ;;

  brain)
    # System-level services.
    for f in "${common_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    for f in "${host_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    reload_systemd_system
    enable_now_system generate_wav.service generate_srt.service generate_mp3.service
    # Keep laravel/other services untouched unless already running.
    try_restart_system ai_generate_fun_facts.service
    try_restart_system generate_wav.service generate_srt.service generate_mp3.service
    ;;

  pinky)
    for f in "${common_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    for f in "${host_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    reload_systemd_system
    enable_now_system generate_wav.service generate_srt.service generate_mp3.service
    try_restart_system gemini_generate_fun_facts.service
    try_restart_system generate_wav.service generate_srt.service generate_mp3.service
    ;;

  legion)
    for f in "${common_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    for f in "${host_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    reload_systemd_system
    enable_now_system generate_wav.service generate_srt.service generate_mp3.service
    try_restart_system generate_wav.service generate_srt.service generate_mp3.service
    ;;

  devbox)
    for f in "${common_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    for f in "${host_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    reload_systemd_system
    # Devbox historically didn't auto-enable these; only try-restart if already running.
    try_restart_system gemini_generate_fun_facts.service
    try_restart_system generate_wav.service generate_srt.service generate_mp3.service
    ;;

  *)
    echo "WARN: unknown host '${host}' - linking all services but not enabling anything" >&2
    for f in "${common_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    for f in "${host_service_dir}"/*.service; do
      [[ -e "$f" ]] || continue
      link_system_service "$f"
    done
    reload_systemd_system
    ;;
esac

echo "Deploy script done host=${host}"
