# About
Karpenter is a Cluster Autoscaling solution optimized for AWS and compatible with EKS, Kops, OpenShift, and other Kubernetes Installations. Karpenter relieves users from the burden of capacity planning on Kubernetes by intelligently adding and removing nodes to match resource demands.

![](./docs/logo.jpeg)
# Usage
## Dependencies

To build Karpenter from source, please first install the following:

1. [go v1.14.4+](https://golang.org/dl/)
2. [kubebuilder dependencies](https://book.kubebuilder.io/quick-start.html#prerequisites)
3. [kubebuilder](https://book.kubebuilder.io/quick-start.html#installation)

## Install

After installing the dependencies from the previous section, build the software:

```bash
make
```

## Build and Push Docker Image

Make sure you have set `KO_DOCKER_REPO` before running the following commands:

```bash
make docker-release
```

If the above works, then you can deploy to your Kubernetes cluster:

```bash
make deploy 
```
