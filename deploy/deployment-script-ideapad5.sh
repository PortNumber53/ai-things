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


echo "-Preparing systemd files"
# Create user systemd directory if it doesn't exist
mkdir -p ~/.config/systemd/user/

# Link service file to user's systemd directory
ln -sfn /deploy/ai-things/current/deploy/ideapad5/systemd/generate_wav.service ~/.config/systemd/user/

# Reload user systemd daemon
systemctl --user daemon-reload


# Enable services
systemctl --user enable --now generate_wav.service

# Restart services
systemctl --user stop generate_wav.service
