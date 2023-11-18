#!/usr/bin/env bash
set -euo pipefail

docker login -u ${DOCKER_USERNAME} -p ${DOCKER_PASSWORD}
# aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin public.ecr.aws
docker build . --progress plain -t ${DOCKER_REPOSITORY}:latest
docker push ${DOCKER_REPOSITORY}:latest
