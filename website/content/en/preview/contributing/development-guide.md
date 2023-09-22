---
title: "Development Guide"
linkTitle: "Development Guide"
weight: 80
description: >
  Set up a Karpenter development environment
---

## Developing

### Setup

Below describe the detailed setup process for getting your development environment ready to deploy and run the dev version of Karpenter.

#### Package Dependencies

The following tools are required for contributing to the Karpenter project.

| Package                                                            | Version  | Install                                        |
| ------------------------------------------------------------------ | -------- | ---------------------------------------------- |
| [go](https://golang.org/dl/)                                       | v1.19+   | [Instructions](https://golang.org/doc/install) |
| [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) |          | `brew install kubectl`                         |
| [helm](https://helm.sh/docs/intro/install/)                        |          | `brew install helm`                            |
| Other tools                                                        |          | `make toolchain`                               |

#### Environment Variables

Karpenter requires certain environment variables be set to use the `Makefile`. Ensure you've set the following values.

```bash
export AWS_ACCOUNT_ID=<account-id>
export AWS_DEFAULT_REGION=<region>
export AWS_SDK_LOAD_CONFIG=true # allows global config to be passed through on local runs
```

#### Container Repository

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

#### Kubernetes Cluster

You can refer to the [Getting Started Guide]({{<ref "../getting-started/getting-started-with-karpenter#create-a-cluster" >}}) for details on how to setup a cluster and IAM permissions to deploy Karpenter.

#### Test Dependencies

Karpenter requires [Kubebuilder's `envtest` dependencies](https://book.kubebuilder.io/reference/envtest.html) to run functional testing. Run the following command to install the test dependencies.

```bash
make toolchain # install test dependencies
```

### Build and Deploy

You can choose to either run the Karpenter controller locally on your machine, pointing to the Kubernetes cluster specified in your `~/.kube/config` or inside the Kubernetes cluster specified in your `~/.kube/config` deployed with [Helm](https://helm.sh/).

#### Locally

This command will run the Go binary locally pointing against the cluster in `~/.kube/config`. Running this command is useful when you need to attach a debugger locally.

```bash
make run # quickly run changes against your cluster
```

#### In a Cluster

```bash
make apply # quickly deploy changes to your cluster
```

If you are only interested in building the Karpenter images and not deploying the updated release to your cluster immediately with Helm, you can run

```bash
make image # build and push the karpenter images
```

### Testing and Formatting

Karpenter CI runs `make presubmit` against the code changes to ensure the changes pass testing and certain formatting, licensing, and security standards.

```bash
make presubmit # run functional testing, add licenses, and format changes
```

```bash
make e2etests # run e2e testing against a live cluster 
```

### Change Log Level

```bash
kubectl patch configmap config-logging -n karpenter --patch '{"data":{"loglevel.controller":"debug"}}' # Debug Level
kubectl patch configmap config-logging -n karpenter --patch '{"data":{"loglevel.controller":"info"}}' # Info Level
```

### Debugging Metrics

OSX:

```bash
open http://localhost:8000/metrics && kubectl port-forward service/karpenter -n karpenter 8000
```

Linux:

```bash
gio open http://localhost:8000/metrics && kubectl port-forward service/karpenter -n karpenter 8000
```

### Tailing Logs

While you can tail Karpenter's logs with kubectl, there's a number of tools out there that enhance the experience. We recommend [Stern](https://pkg.go.dev/github.com/planetscale/stern#section-readme):

```bash
stern -n karpenter -l app.kubernetes.io/name=karpenter
```

Finally, to deploy the correct IAM permissions, including the instance profile for provisioned nodes, run

```bash
make setup
```

## Profiling memory

Karpenter exposes a pprof endpoint on its metrics port.

Learn about profiling with pprof: https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/

### Prerequisites

```bash
brew install graphviz
go install github.com/google/pprof@latest
```

### Get a profile

```bash
# Connect to the metrics endpoint
kubectl port-forward service/karpenter -n karpenter 8000
open http://localhost:8000/debug/pprof/
# Visualize the memory
go tool pprof -http 0.0.0.0:9000 localhost:8000/debug/pprof/heap
```
