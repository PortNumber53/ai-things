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
sudo rsync -ravp --progress ./deploy/brain/systemd/ /etc/systemd/system/
sudo systemctl daemon-reload



cd $DEPLOY_BASE_PATH

sudo ln -sfn $DEPLOYMENT_RELEASE_PATH$TIMESTAMP current
