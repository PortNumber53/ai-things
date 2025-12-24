#!/bin/bash

set -x
set -e

hostname
pwd


# Capture parameters into variables
DEPLOY_BASE_PATH=$1
DEPLOYMENT_RELEASE_PATH=$2
DEPLOYMENT_PATH=$3
TIMESTAMP=$4

echo "Updating release symlink"

cd ${DEPLOY_BASE_PATH}
ln -sfn ${DEPLOYMENT_PATH} ./current

echo "-Preparing Python environments"
bash /deploy/ai-things/current/deploy/setup_python_envs.sh "${DEPLOY_BASE_PATH}" "${DEPLOY_BASE_PATH%/}/current"


echo "-Preparing systemd files"
cd /etc/systemd/system/

sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/ai_generate_fun_facts.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/generate_wav.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/generate_srt.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/generate_mp3.service

sudo systemctl daemon-reload

# Enable services
sudo systemctl enable --now generate_wav.service
sudo systemctl enable --now generate_srt.service
sudo systemctl enable --now generate_mp3.service

# Restart services
sudo systemctl stop ai_generate_fun_facts.service
sudo systemctl stop generate_wav.service
sudo systemctl stop generate_srt.service
sudo systemctl stop generate_mp3.service
