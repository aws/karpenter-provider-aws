# Contributing

## Dependencies

The following tools are required for doing development on Karpenter.

| Package                                                            | Version  | Install             |
| ------------------------------------------------------------------ | -------- | ------------------- |
| [go](https://golang.org/dl/)                                       | v1.14.4+ |                     |
| [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) |          |                     |
| [helm](https://helm.sh/docs/intro/install/)                        |          | `brew install helm` |
| Other tools                                                        |          | `make toolchain`    |

## Developing

### Setup / Teardown

```
./hack/quick-install.sh          # Install cluster dependencies and karpenter
./hack/quick-install.sh --delete # Clean everything up
```

### Developer Loop

Local development is not supported at this time.

* Make sure dependencies are installed
* You will need a personal development image repository (e.g. ECR)
* $KO_DOCKER_REPO must point to your development repository
* Your cluster must have permissions to read from the repository

### Build and Deploy
```
make        # build and test code
make deploy # deploy local changes to cluster
```

### Testing
```
make test       # E2e correctness tests
make battletest # More rigorous tests run in CI environment
```

### Debugging Metrics
Prometheus
```
open http://localhost:9090/graph && kubectl port-forward service/prometheus-operated -n karpenter 9090
```
Karpenter Metrics
```
open http://localhost:8080/metrics && kubectl port-forward service/karpenter-metrics-service -n karpenter 8080
```

## AWS

### Setting up a development repository with ECR
Follow the ECR getting started guide and create a development repository with [these instructions](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html). Then configure your shell to with your newly created repository
```
export DEVELOPMENT_REPO="${AWS_ACCOUNT_ID}.dkr.ecr.us-west-2.amazonaws.com"
export KO_DOCKER_REPO=${DEVELOPMENT_REPO}
aws ecr get-login-password --region us-west-2 | docker login --username AWS --password-stdin $DEVELOPMENT_REPO
```
