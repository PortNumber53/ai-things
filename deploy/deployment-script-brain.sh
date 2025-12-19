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

# Disable servies
sudo systemctl disable --now laravel-worker@text_fun_facts.service


echo "Updating release symlink"
cd ${DEPLOYMENT_PATH}/manager
# Keep logs within the release path to avoid relying on shared storage.
mkdir -p storage/app/public storage/framework/cache storage/framework/sessions storage/framework/views storage/framework/testing storage/logs

cd ${DEPLOY_BASE_PATH}
ln -sfn ${DEPLOYMENT_PATH} ./current


echo "-Preparing systemd files"
cd /etc/systemd/system/

sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/laravel-worker@.service laravel-worker@text_fun_facts.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/ai_generate_fun_facts.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/generate_wav.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/generate_srt.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/generate_mp3.service

sudo systemctl daemon-reload

# Run migrations
cd ${DEPLOYMENT_PATH}/manager
composer install --no-ansi

# Enable services
sudo systemctl disable --now laravel-worker@text_fun_facts.service
sudo systemctl enable --now generate_wav.service
sudo systemctl enable --now generate_srt.service
sudo systemctl enable --now generate_mp3.service

# Restart services
sudo systemctl stop ai_generate_fun_facts.service
sudo systemctl stop laravel-worker@text_fun_facts.service
sudo systemctl stop generate_wav.service
sudo systemctl stop generate_srt.service
sudo systemctl stop generate_mp3.service
