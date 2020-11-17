# Contributing

## Dependencies

The following tools are required for doing development on Karpenter.

| Package                                                            | Version  | Install                |
| ------------------------------------------------------------------ | -------- | ---------------------- |
| [go](https://golang.org/dl/)                                       | v1.15.3+ | `brew install go`      |
| [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) |          | `brew install kubectl` |
| [helm](https://helm.sh/docs/intro/install/)                        |          | `brew install helm`    |
| Other tools                                                        |          | `make toolchain`       |

## Developing

### Setup / Teardown

Based on which environment you are running a Kubernetes cluster, follow the [Environment specific setup](##Environment-specific-setup) for setting up your environment before you continue. Once you have the environment specific settings, to install Karpenter in a Kubernetes cluster run the following commands.

```
make generate                    # Create auto-generated YAML files.
./hack/quick-install.sh          # Install cluster dependencies and karpenter
./hack/quick-install.sh --delete # Clean everything up
```

### Developer Loop

* Make sure dependencies are installed
    * Run `make generate` to make sure yaml manifests are generated
    * Run `make toolchain` to install cli tools for building and testing the project
* You will need a personal development image repository (e.g. ECR)
    * Make sure you have valid credentials to your development repository.
    * `$KO_DOCKER_REPO` must point to your development repository
    * Your cluster must have permissions to read from the repository
* Make sure your cluster doesn't have previous installations of prometheus and cert-manager
  * Previous installations of our dependencies can interfere with our installation scripts, so to be safe, clear those, then run `./hack/quick-install.sh`
* If running `./hack/quick-install.sh` fails with `Error: Accumulate Target`, run `make generate` successfully, and try again.

### Build and Deploy
```
make all                                  # build and test code
make apply                                # deploy local changes to cluster
CLOUD_PROVIDER=<YOUR_PROVIDER> make apply # deploy for your cloud provider
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
    --repository-name karpenter \
    --image-scanning-configuration scanOnPush=true \
    --region ${REGION}
```

Once you have your ECR repository provisioned, configure your Docker daemon to authenticate with your newly created repository.

```
export KO_DOCKER_REPO="${AWS_ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
aws ecr get-login-password --region ${REGION} | docker login --username AWS --password-stdin $KO_DOCKER_REPO
```
