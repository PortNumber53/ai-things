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
cd ${DEPLOYMENT_PATH}/api
ln -sfn ${DEPLOY_BASE_PATH}storage storage

cd ${DEPLOY_BASE_PATH}
ln -sfn ${DEPLOYMENT_PATH} ./current


echo "-Preparing systemd files"
cd /etc/systemd/system/

sudo ln -sfn /deploy/ai-things/current/deploy/pinky/systemd/convert_to_mp3.service convert_to_mp3.service
sudo ln -sfn /deploy/ai-things/current/deploy/pinky/systemd/gemini_generate_fun_facts.service gemini_generate_fun_facts.service
sudo ln -sfn /deploy/ai-things/current/deploy/pinky/systemd/generate_wav.service
sudo ln -sfn /deploy/ai-things/current/deploy/pinky/systemd/generate_srt.service

sudo systemctl daemon-reload

# Run migrations
cd ${DEPLOYMENT_PATH}/api
composer install --no-ansi
./artisan migrate --force


# Enable services
sudo systemctl enable --now gemini_generate_fun_facts.service
sudo systemctl enable --now generate_wav.service
# sudo systemctl disable --now convert_to_mp3.service

# Restart services
sudo systemctl restart generate_wav.service
sudo systemctl restart generate_srt.service
sudo systemctl stop gemini_generate_fun_facts.service
sudo systemctl stop convert_to_mp3.service
