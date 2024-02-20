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


echo "Updating current symlink"
cd $DEPLOY_BASE_PATH
sudo ln -sfn $DEPLOYMENT_RELEASE_PATH$TIMESTAMP current


cd ${DEPLOYMENT_PATH}
ln -sfn ${DEPLOY_BASE_PATH}storage storage

echo "-Preparing systemd files"
cd /etc/systemd/system/

sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/laravel-worker@.service laravel-worker@text_fun_facts.service
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/ai_generate_fun_facts.service
sudo systemctl daemon-reload

# Enable services
sudo systemctl enable --now laravel-worker@text_fun_facts.service

# Restart services
sudo systemctl restart laravel-worker@text_fun_facts.service
