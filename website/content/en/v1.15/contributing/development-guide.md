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

If you are only interested in building the Karpenter images and not deploying the updated release to your cluster immediately with Helm, you can run the following:

```bash
make image # build and push the karpenter images
```

*Note: that this will produce a build with the version of https://github.com/kubernetes-sigs/karpenter in your local filesystem.

You can test out changes made in https://github.com/kubernetes-sigs/karpenter by replacing the dependency of https://github.com/aws/karpenter-provider-aws/.
For local changes, replace `$PATH_TO_KUBERNETES_SIGS_KARPENTER` with the relative or absolute path and run:

```bash
go mod edit -replace sigs.k8s.io/karpenter=$PATH_TO_KUBERNETES_SIGS_KARPENTER
```

Then you can build your image using the previous steps.

### Publishing Images Only

If you only need to build and publish an image to a container registry, run the following:

```bash
make image # build and push the karpenter images
```
You can test out changes made in https://github.com/kubernetes-sigs/karpenter by replacing the dependency of https://github.com/aws/karpenter-provider-aws/.
For local changes, replace `$PATH_TO_KUBERNETES_SIGS_KARPENTER` with the relative or absolute path and run:

```bash
go mod edit -replace sigs.k8s.io/karpenter=$PATH_TO_KUBERNETES_SIGS_KARPENTER
```

Then you can build your image using the previous steps.

*Note: you need to commit the go.mod changes before running `make image` for the changes to be picked up.

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

## Profiling
Karpenter exposes a pprof endpoint on its metrics port when [profiling]({{< relref "../reference/settings" >}}) is enabled.

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
# Visualize CPU
go tool pprof -http 0.0.0.0:9000 "localhost:8080/debug/pprof/profile?seconds=60"
```
