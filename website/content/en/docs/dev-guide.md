---
title: "Development Guide"
linkTitle: "Development Guide"
weight: 80
---

## Dependencies

The following tools are required for contributing to the Karpenter project.

| Package                                                            | Version  | Install                |
| ------------------------------------------------------------------ | -------- | ---------------------- |
| [go](https://golang.org/dl/)                                       | v1.15.3+ | `brew install go`      |
| [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) |          | `brew install kubectl` |
| [helm](https://helm.sh/docs/intro/install/)                        |          | `brew install helm`    |
| Other tools                                                        |          | `make toolchain`       |

## Developing

### Setup / Teardown

Based on how you are running your Kubernetes cluster, follow the [Environment specific setup](#environment-specific-setup) to configure your environment before you continue. Once you have your environment set up, to install Karpenter in the Kubernetes cluster specified in your `~/.kube/config`  run the following commands.

```
make codegen # Create auto-generated YAML files.
make apply # Install Karpenter
make delete # Uninstall Karpenter
```

### Developer Loop
* Make sure dependencies are installed
    * Run `make codegen` to make sure yaml manifests are generated
    * Run `make toolchain` to install cli tools for building and testing the project
* You will need a personal development image repository (e.g. ECR)
    * Make sure you have valid credentials to your development repository.
    * `$KO_DOCKER_REPO` must point to your development repository
    * Your cluster must have permissions to read from the repository
* If you created your cluster on version 1.19 or above, you may need to tag your subnets as mentioned [here](docs/aws/README.md). This is a temporary problem with our subnet discovery system, and is being tracked [here](https://github.com/awslabs/karpenter/issues/404#issuecomment-845283904).

### Build and Deploy
```
make dev                                  # build and test code
make apply                                # deploy local changes to cluster
CLOUD_PROVIDER=<YOUR_PROVIDER> make apply # deploy for your cloud provider
```

### Testing
```
make test       # E2e correctness tests
make battletest # More rigorous tests run in CI environment
```

### Verbose Logging
```bash
kubectl patch deployment karpenter -n karpenter --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["--verbose"]}]'
```

### Debugging Metrics
```bash
open http://localhost:8080/metrics && kubectl port-forward service/karpenter-metrics -n karpenter 8080
```

## Environment specific setup

### AWS
Set the CLOUD_PROVIDER environment variable to build cloud provider specific packages of Karpenter.

```
export CLOUD_PROVIDER=aws
```

For local development on Karpenter you will need a Docker repo which can manage your images for Karpenter components.
You can use the following command to provision an ECR repository.
```
aws ecr create-repository \
    --repository-name karpenter/controller \
    --image-scanning-configuration scanOnPush=true \
    --region ${AWS_DEFAULT_REGION}
```

Once you have your ECR repository provisioned, configure your Docker daemon to authenticate with your newly created repository.

```
export KO_DOCKER_REPO="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/karpenter"
aws ecr get-login-password --region ${AWS_DEFAULT_REGION} | docker login --username AWS --password-stdin $KO_DOCKER_REPO
```
