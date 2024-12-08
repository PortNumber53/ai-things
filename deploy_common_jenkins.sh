#!/bin/bash

## This script is used from Jenkins to deploy to a target host

echo "BRANCH_NAME: ${BRANCH_NAME}"
echo "TARGET_HOST: ${TARGET_HOST}"

# Allowed target hosts
ALLOWED_HOSTS=("brain" "macbook" "pinky" "web1" "ideapad5")

# Check if TARGET_HOST is valid
if [[ ! " ${ALLOWED_HOSTS[@]} " =~ " ${TARGET_HOST} " ]]; then
  echo "Error: Invalid TARGET_HOST '${TARGET_HOST}'. Allowed values are: ${ALLOWED_HOSTS[*]}"
  exit 1
fi

# Rsync current folder to target host
rsync -avz --exclude 'storage' --exclude 'bootstrap/cache' \
  --exclude 'public/storage' --exclude 'vendor' \
  --exclude 'node_modules' --exclude 'package-lock.json' \
  --exclude 'yarn.lock' --exclude 'package.json' \
  --exclude 'composer.lock' --exclude 'package.json' \
  --exclude 'manager/storage' --exclude 'manager/vendor' --exclude 'manager/node_modules' \
  --exclude 'manager/public/storage' --exclude 'manager/bootstrap/cache' \
  --exclude 'manager/node_modules' --exclude 'manager/vendor' \
  --exclude 'manager/storage' --exclude 'manager/vendor' --exclude 'manager/node_modules' \ grimlock@${TARGET_HOST}:/deploy/ai-things/current/

SCRIPT_NAME="deploy_${TARGET_HOST}.sh"
# SSH into the target host and run the deployment script for that host
ssh grimlock@${TARGET_HOST} "cd /deploy/ai-things/current/deploy && ./${SCRIPT_NAME}"
