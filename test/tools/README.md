Tools is an image that contains binaries useful for testing karpenter.

```
aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin public.ecr.aws
docker build ./test/tools --progress plain -t public.ecr.aws/karpenter-testing/tools:latest
docker push public.ecr.aws/karpenter-testing/tools:latest
```
