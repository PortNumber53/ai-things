#!/bin/bash

echo "Deploying ideapad5"


df -h
ls -la

set -x
set -e



hostname
pwd
ls -la


export DEPLOY_BASE_PATH="/deploy/ai-things/"
export DEPLOYMENT_RELEASE_PATH="${DEPLOY_BASE_PATH}releases/"
export DEPLOYMENT_PATH="${DEPLOYMENT_RELEASE_PATH}$(date +%Y%m%d%H%M%S)"
export TIMESTAMP=$(date +%Y%m%d%H%M%S)

mkdir -p ${DEPLOYMENT_PATH}

# Rsync the current directory to the deployment path
rsync -avz --exclude 'storage' --exclude 'bootstrap/cache' \
    --exclude 'public/storage' --exclude 'vendor' \
    --exclude 'node_modules' --exclude 'package-lock.json' \
    --exclude 'yarn.lock' --exclude 'package.json' \
    --exclude 'composer.lock' --exclude 'composer.json' \
    --exclude 'package.json' \
    --exclude 'storage' --exclude 'bootstrap/cache' \
    --exclude 'public/storage' --exclude 'vendor' \
    --exclude 'node_modules' --exclude 'package-lock.json' \
    --exclude 'yarn.lock' --exclude 'package.json' \
    --exclude 'composer.lock' --exclude 'composer.json' \

# Install dependencies
cd ${DEPLOYMENT_PATH}
composer install --no-ansi
npm install
npm run build
cd ..

# We want to symlink deploy/systemd/generate_wav.service to the users systemd folder
# Create user systemd directory if it doesn't exist
mkdir -p ~/.config/systemd/user/

# Link service file to user's systemd directory
ln -sfn /deploy/ai-things/current/deploy/ideapad5/systemd/generate_wav.service ~/.config/systemd/user/

# Reload user systemd daemon
systemctl --user daemon-reload

# Check if generate_wav.service exists and restart it if so
if systemctl --user list-unit-files | grep -q generate_wav.service; then
    systemctl --user restart generate_wav.service
else
    echo "generate_wav.service not found"
fi

# Enable generate_wav.service
systemctl --user enable generate_wav.service
