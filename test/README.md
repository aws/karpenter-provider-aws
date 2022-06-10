# Testing Directory

All files and configurations used for testing as detailed in the [Testing Guide](https://karpenter.sh/preview/testing-guide/) live here. Configurations and setup files here will be used in the future to test PRs.

## File Directory
- `/infrastructure`: Testing infrastructure
- `/suites`: [Tekton](https://tekton.dev/) CRDs defining test suites

## Testing Infrastructure Design Choices
Testing infrastructure will be divided up into three layers: Management Cluster, Test Orchestration, and Clusters Under Test. The Management Cluster will be a Kubernetes cluster with configured add-ons to run a Test Orchestration tool that will create Clusters Under Test where Karpenter will be tested.
- `Management Cluster`: An EKS Cluster with configured Add-ons and Tekton
- `Test Orchestration`: Tekton to create Clusters Under Test and run Test Suites.
- `Clusters Under Test`: Rapid iteration [KIT](https://github.com/awslabs/kubernetes-iteration-toolkit) Guest Clusters and EKS Clusters where test suites will run.

*Note: A more formal design discussing testing infrastructure will come soon.*
