## Tekton Examples

The tekton testing configurations here are meant to show how to utilize some of Tekton's features and how document some core workflows that will be used for CI tests.

Contents:
- `hello-world.yaml`: Simple definitions of a Task, Pipeline, and PipelineRun
- `shared-hello-world.yaml`: Demonstration of workspaces to share information between tasks
- `create-eks.yaml`: Create an EKS Cluster with eksctl
- `create-kit.yaml`: Create a [KIT](https://github.com/awslabs/kubernetes-iteration-toolkit) Guest Cluster
- `install-karpenter.yaml`: Install Karpenter on a cluster
