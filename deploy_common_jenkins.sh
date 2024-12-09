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

# Map TARGET_HOST to credential ID and env file name
case "${TARGET_HOST}" in
  "brain")
    CRED_ID="ai-things-brain-env-prod-file"
    ;;
  "pinky")
    CRED_ID="ai-things-pinky-env-prod-file"
    ;;
  "legion")
    CRED_ID="ai-things-legion-env-prod-file"
    ;;
  "devbox")
    CRED_ID="ai-things-devbox-env-prod-file"
    ;;
  *)
    echo "No specific .env file configured for ${TARGET_HOST}"
     ;;
esac

# Copy the appropriate .env file if configured for this host
if [ -n "$CRED_ID" ]; then
  if [ -f "$ENV_FILE_SOURCE" ]; then
    cp --no-preserve=mode,ownership "$ENV_FILE_SOURCE" .env
    echo "Copied .env file for ${TARGET_HOST}"
  else
    echo "Error: ENV_FILE_SOURCE not found or not accessible"
    exit 1
  fi
else
  echo "No specific .env file configured for ${TARGET_HOST}"
fi

# Rsync workspace folder to release folder on the target host
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
