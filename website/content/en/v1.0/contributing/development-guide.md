---
title: "Development Guide"
linkTitle: "Development Guide"
weight: 80
description: >
  Set up a Karpenter development environment
---

## Dependencies

The following tools are required for contributing to the Karpenter project.

| Package                                                            | Version  | Install                                        |
| ------------------------------------------------------------------ | -------- | ---------------------------------------------- |
| [go](https://golang.org/dl/)                                       | v1.19+   | [Instructions](https://golang.org/doc/install) |
| [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) |          | `brew install kubectl`                         |
| [helm](https://helm.sh/docs/intro/install/)                        |          | `brew install helm`                            |
| Other tools                                                        |          | `make toolchain`                               |

## Developing

### Setup / Teardown

Based on how you are running your Kubernetes cluster, follow the [Environment specific setup](#environment-specific-setup) to configure your environment before you continue. You can choose to either run the Karpenter controller locally on your machine, pointing to the Kubernetes cluster specified in your `~/.kube/config` or inside the Kubernetes cluster specified in your `~/.kube/config` deployed with [Helm](https://helm.sh/).

#### Locally

Once you have your environment set up, run the following commands to run the Karpenter Go binary against the Kubernetes cluster specified in your `~/.kube/config`

```bash
make run
```

#### Inside a Kubernetes Cluster

Once you have your environment set up, to install Karpenter in the Kubernetes cluster specified in your `~/.kube/config`  run the following commands.

```bash
make apply # Install Karpenter
make delete # Uninstall Karpenter
```

### Developer Loop

* Make sure dependencies are installed
    * Run `make codegen` to make sure yaml manifests are generated (requires a working set of AWS credentials, see [Specifying Credentials](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials))
    * Run `make toolchain` to install cli tools for building and testing the project
* You will need a personal development image repository (e.g. ECR)
    * Make sure you have valid credentials to your development repository.
    * `$KO_DOCKER_REPO` must point to your development repository
    * Your cluster must have permissions to read from the repository

### Build and Deploy

*Note: these commands do not rely on each other and may be executed independently*

```bash
make apply # quickly deploy changes to your cluster
make presubmit # run codegen, lint, and tests
```

If you are only interested in building the Karpenter images and not deploying the updated release to your cluster immediately with Helm, you can run

```bash
make image # build and push the karpenter images
```

### Testing

```bash
make test       # E2E correctness tests
```

### Change Log Level

By default, `make apply` will set the log level to debug. You can change the log level by setting the log level in your Helm values.

```bash
--set logLevel=debug
```

### Debugging Metrics

OSX:

```bash
open http://localhost:8080/metrics && kubectl port-forward service/karpenter -n kube-system 8080
```

Linux:

```bash
gio open http://localhost:8080/metrics && kubectl port-forward service/karpenter -n karpenter 8080
```

### Tailing Logs

While you can tail Karpenter's logs with kubectl, there's a number of tools out there that enhance the experience. We recommend [Stern](https://pkg.go.dev/github.com/planetscale/stern#section-readme):

```bash
stern -n karpenter -l app.kubernetes.io/name=karpenter
```

## Environment specific setup

### AWS

For local development on Karpenter you will need a Docker repo which can manage your images for Karpenter components.
You can use the following command to provision an ECR repository. We recommend using a single "dev" repository for 
development across multiple projects, and to use specific image hashes instead of image tags. 

```bash
aws ecr create-repository \
    --repository-name dev \
    --image-scanning-configuration scanOnPush=true \
    --region "${AWS_DEFAULT_REGION}"
```

Once you have your ECR repository provisioned, configure your Docker daemon to authenticate with your newly created repository.

```bash
export KO_DOCKER_REPO="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/dev"
aws ecr get-login-password --region "${AWS_DEFAULT_REGION}" | docker login --username AWS --password-stdin "${KO_DOCKER_REPO}"
```

Finally, to deploy the correct IAM permissions, including the instance profile for provisioned nodes, run

```bash
make setup
```

## Profiling memory
Karpenter exposes a pprof endpoint on its metrics port.

Learn about profiling with pprof: https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/

### Prerequisites
```
brew install graphviz
go install github.com/google/pprof@latest
```

### Get a profile
```
# Connect to the metrics endpoint
kubectl port-forward service/karpenter -n karpenter 8080
open http://localhost:8080/debug/pprof/
# Visualize the memory
go tool pprof -http 0.0.0.0:9000 localhost:8080/debug/pprof/heap
```
