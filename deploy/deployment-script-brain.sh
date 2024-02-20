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


echo "copying systemd files"
cd /etc/systemd/system/
sudo ln -sfn /deploy/ai-things/current/deploy/brain/systemd/ai_generate_fun_facts.service
sudo systemctl daemon-reload



cd $DEPLOY_BASE_PATH

sudo ln -sfn $DEPLOYMENT_RELEASE_PATH$TIMESTAMP current
