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

## Setting up Container Repository

### Using AWS ECR

If you plan on  using the AWS ECR and haven't yet set it up, you will want to do something like the following (which come from [these instructions](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html)

```bash
# Replace the values in the following 2 lines:
AWS_ACCOUNT_ID=fillThisIn
AWS_REGION=us-west-2
aws ecr get-login-password --region $AWS_ACCOUNT_ID | docker login --username AWS --password-stdin ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com
aws ecr create-repository --repository-name karpenter --image-scanning-configuration scanOnPush=true --region ${AWS_REGION}
```

You will then want to add the following to your shell's init script:

```bash
export KO_DOCKER_REPO=${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com
```
## Build and Push Docker Image

Make sure you have set `KO_DOCKER_REPO` before running the following commands:

```bash
make docker-build && make docker-push
```

If the above works, then you can deploy to your Kubernetes cluster:

```bash
make deploy 
```
