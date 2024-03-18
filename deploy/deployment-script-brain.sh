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
cd ${DEPLOYMENT_PATH}/api
ln -sfn ${DEPLOY_BASE_PATH}storage storage

cd ${DEPLOY_BASE_PATH}
ln -sfn ${DEPLOYMENT_PATH} ./current


echo "-Preparing systemd files"
cd /etc/systemd/system/

sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/laravel-worker@.service laravel-worker@text_fun_facts.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/ai_generate_fun_facts.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/generate_wav.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/generate_srt.service

sudo systemctl daemon-reload

# Run migrations
cd ${DEPLOYMENT_PATH}/api
composer install --no-ansi

# Enable services
sudo systemctl disable --now laravel-worker@text_fun_facts.service
# sudo systemctl disable --now generate_wav.service

# Restart services
sudo systemctl stop ai_generate_fun_facts.service
sudo systemctl stop laravel-worker@text_fun_facts.service
sudo systemctl restart generate_wav.service
sudo systemctl restart generate_srt.service
