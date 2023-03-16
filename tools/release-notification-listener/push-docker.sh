#!/usr/bin/env bash
set -euo pipefail

aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin public.ecr.aws
docker build ./test/cmd/release-notification-listener --progress plain -t public.ecr.aws/karpenter/release-notification-listener:latest
docker push public.ecr.aws/karpenter/release-notification-listener:latest
