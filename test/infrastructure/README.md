# Karpenter Test Infrastructure

Karpenter's test infrastructure utilizes the [Kubernetes Iteration Toolkit (KIT)](https://github.com/awslabs/kubernetes-iteration-toolkit/tree/main/infrastructure) Infrastructure which uses the [Cloud Development Kit (CDK)](https://docs.aws.amazon.com/cdk/v2/guide/home.html) to provision a host K8s cluster. The host cluster has some add-ons such as the AWS VPC CNI, EBS CSI Driver, Karpenter, AWS Load Balancer Controller, KIT Operator, and Flux. Flux is configured to monitor two git repositories. 

The first is the KIT repo to install some additional add-ons that do not require IAM Roles for Service Accounts (IRSA) such as Tekton, Prometheus, Metrics-Server, etc. 

The second repo is the "application-in-test" repo which, in this case, is Karpenter. Flux monitors the `test/infrastructure/clusters/test-infra` directory for K8s manifests. Minimally, the manifests stored at the flux sync path is a Kustomize flux spec that points at the `test/suites` directory. Additional K8s manifests that the Karpenter host cluster needs can be placed in this directory.

## Setup

The Karpenter team will operate a host cluster for testing, but it may also be useful to standup your own host cluster to test with or to develop tests. 

The Karpenter team's host cluster is created with the following script:

```
git clone git@github.com:awslabs/kubernetes-iteration-toolkit.git
cd kubernetes-iteration-toolkit/infrastructure
cdk bootstrap
cdk deploy KITInfrastructure \
 -c TestFluxRepoName="karpenter" \
 -c TestFluxRepoURL="https://github.com/aws/karpenter" \
 -c TestFluxRepoBranch="main" \
 -c TestFluxRepoPath="./test/infrastructure/clusters/test-infra" \
 -c TestNamespace="karpenter-tests" \
 -c TestServiceAccount="karpenter-tests"
```

The CDK command will output a command to update your kubeconfig so that you can interact with the cluster:

```
aws eks update-kubeconfig --name KITInfrastructure --role-arn <> .....
```

You can create your own host cluster pointing at your fork by modifying the `TestFluxRepoURL` to point at your fork and the  `TestFluxRepoBranch` to point at a different branch:

```
git clone git@github.com:awslabs/kubernetes-iteration-toolkit.git
cd kubernetes-iteration-toolkit/infrastructure
cdk bootstrap
cdk deploy KITInfrastructure \
 -c TestFluxRepoName="karpenter" \
 -c TestFluxRepoURL="https://github.com/${my-fork}/karpenter" \
 -c TestFluxRepoBranch="${my-feature-branch}" \
 -c TestFluxRepoPath="./test/infrastructure/clusters/test-infra" \
 -c TestNamespace="karpenter-tests" \
 -c TestServiceAccount="karpenter-tests"
```

## Executing Tests:

Test execution will generally start by provisioning another EKS cluster or KIT guest cluster so that the tests are run in isolation. The cluster creation process can take a while.

To run tests, you can use the web UI or the Tekton CLI `tkn`:

### Web UI:

```
kubectl port-forward service/tekton-dashboard -n tekton-pipelines 9097 &
open localhost:9097
```

### Tekton CLI:

```
## List available pipelines:
tkn pipeline list -n karpenter-tests

## Start a pipeline
tkn pipeline start suite --namespace karpenter-tests --serviceaccount karpenter-tests --showlog --use-param-defaults --param 'test-filter=Integration'
```
