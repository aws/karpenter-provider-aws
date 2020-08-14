# About
Karpenter is a Cluster Autoscaling solution optimized for AWS and compatible with EKS, Kops, OpenShift, and other Kubernetes Installations. Karpenter relieves users from the burden of capacity planning on Kubernetes by intelligently adding and removing nodes to match resource demands.

![](./docs/logo.jpeg)
# Usage
## Dependencies

1. [go v1.14.4+](https://golang.org/dl/)
2. [docker](https://docs.docker.com/install/)

## Install

Make sure you have set `KO_DOCKER_REPO` before running the following commands:

```bash
make docker-release
```

If the above works, then you can deploy to your Kubernetes cluster:

```bash
make deploy 
```
