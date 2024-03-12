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

sudo ln -sfn /deploy/ai-things/current/deploy/pinky/systemd/gemini_generate_fun_facts.service gemini_generate_fun_facts.service
sudo ln -sfn /deploy/ai-things/current/deploy/pinky/systemd/gemini_generate_fun_facts.service
sudo systemctl daemon-reload

# Run migrations
cd ${DEPLOYMENT_PATH}/api
composer install --no-ansi
./artisan migrate --force


# Enable services
sudo systemctl enable --now gemini_generate_fun_facts.service

# Restart services
sudo systemctl restart gemini_generate_fun_facts.service
