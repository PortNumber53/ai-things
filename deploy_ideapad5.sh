#!/bin/bash

echo "Deploying ideapad5"
# list first and second parameters received from Jenkins

RELEASE_FOLDER=$1
TIMESTAMP=$2
echo "RELEASE_FOLDER: ${RELEASE_FOLDER}"
echo "TIMESTAMP: ${TIMESTAMP}"

df -h
ls -la

set -x
set -e



hostname
pwd
ls -la



# Install dependencies
cd ${RELEASE_FOLDER}/manager

# Create folders required my Laravel
mkdir -pv storage bootstrap/cache

composer install --no-ansi
npm install
cd ..

# We want to symlink deploy/systemd/generate_wav.service to the users systemd folder
# Create user systemd directory if it doesn't exist
mkdir -p ~/.config/systemd/user/


# Link release folder to current folder
cd /deploy/ai-things
ln -sfn ${RELEASE_FOLDER} ./current

# Link service file to user's systemd directory
cd /deploy/ai-things/current
ln -sfn /deploy/ai-things/current/deploy/systemd/generate_wav.service ~/.config/systemd/user/
ln -sfn /deploy/ai-things/current/deploy/systemd/generate_srt.service ~/.config/systemd/user/

# Reload user systemd daemon
systemctl --user daemon-reload

# Check if generate_wav.service exists and restart it if so
if systemctl --user list-unit-files | grep -q generate_wav.service; then
    echo "restarting generate_wav.service"
    systemctl --user restart generate_wav.service
else
    echo "generate_wav.service not found"
fi


# Check if generate_srt.service exists and restart it if so
if systemctl --user list-unit-files | grep -q generate_srt.service; then
    echo "restarting generate_srt.service"
    systemctl --user restart generate_srt.service
else
    echo "generate_srt.service not found"
fi

# Enable generate_wav.service
systemctl --user enable generate_wav.service

# Enable generate_srt.service
systemctl --user enable generate_srt.service
