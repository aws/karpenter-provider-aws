# Testing Directory README

All files and configurations used for testing as detailed in the [Testing Guide](https://karpenter.sh/preview/testing-guide/) live here. Configurations and setup files here will be used in the future to test PRs.

*Note: Currently, the configurations and examples in this folder should not be used in production or in local setups.*

## File Directory
- `/images`: Dockerfiles that Tekton task pods will run
- `/setup`: Testing infrastructure
- `/tekton`: Tekton YAMLs defining test suites

## Design and Framework Choices
Testing infrastructure will be divided up into three layers. Host Cluster, Workflow Management, and Test Clusters. The Host Cluster will be a Kubernetes cluster to run a Workflow Management tool that will create Test Clusters where Karpenter will be tested.
- `Host Cluster`: An EKS Cluster to orchestrate Tekton
- `Workflow Management`: Tekton yamls to create Test Clusters
- `Test Cluster`: [KIT](https://github.com/awslabs/kubernetes-iteration-toolkit) Guest Clusters to test Karpenter

*Note: A more formal design discussing testing infrastructure will come soon.*
