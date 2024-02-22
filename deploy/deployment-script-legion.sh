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
