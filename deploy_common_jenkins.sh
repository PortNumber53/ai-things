#!/bin/bash

set -x
set -e

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

BASE_DEPLOY_FOLDER="/deploy/ai-things/"
## Create a timestamped folder for the release
TIMESTAMP=$(date +%Y%m%d%H%M%S)
RELEASE_FOLDER="${BASE_DEPLOY_FOLDER}releases/${TIMESTAMP}"
# Create the release folder on the target host
ssh grimlock@${TARGET_HOST} "mkdir -pv ${RELEASE_FOLDER}"
pwd

# Rsync workspace tolder to release folder on the target host
rsync -avz \
  --exclude '.git' \
  --exclude 'storage' --exclude 'bootstrap/cache' \
  --exclude 'public/storage' --exclude 'vendor' \
  --exclude 'node_modules' \
  --exclude 'manager/storage' --exclude 'manager/vendor' --exclude 'manager/node_modules' \
  --exclude 'manager/public/storage' --exclude 'manager/bootstrap/cache' \
  ./ grimlock@${TARGET_HOST}:${RELEASE_FOLDER}/

SCRIPT_NAME="deploy_${TARGET_HOST}.sh"
# SSH into the target host and run the deployment script for that host
ssh grimlock@${TARGET_HOST} "cd ${RELEASE_FOLDER} && ls -la &&./${SCRIPT_NAME} ${RELEASE_FOLDER} ${TIMESTAMP}"
