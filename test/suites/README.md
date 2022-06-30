# Test Suites

## Purpose
Test Suites will be used to test Karpenter in Clusters Under Test. These configurations are not meant to run production workloads. All test suites will be defined as [Tekton](https://tekton.dev/) CRDs.

## File Directory
- `examples`: Example Tekton configs that will be used in test suites.
  - `create-eks.yaml`: Create an EKS Cluster with eksctl
  - `create-kit.yaml`: Create a [KIT](https://github.com/awslabs/kubernetes-iteration-toolkit) Guest Cluster
  - `install-karpenter.yaml`: Install Karpenter on a cluster given a kubeconfig

## Tekton Concepts
Tekton CRDs used here are distinguished as Tasks, Pipelines, and PipelineRuns. Tasks are ordered and parameterized by Pipelines. Pipelines are instantiated by PiplineRuns.
- `Tasks` can be split into steps, where each task will be created in Kubernetes as a pod, and each step as a container. Users can parameterize `Tasks` through `Tasks`, `TaskRuns`, `Pipelines`, or `PipelineRuns`.
- `Pipelines` define orders of `Tasks` and configure `Workspaces` for intra- and inter- `Task` communication.
- `TaskRuns` and `PipelineRuns` are used to initiate `Tasks` and `Pipelines`, respectively.

### How to run with the [Tekton CLI](https://github.com/tektoncd/cli):
After creating these resources using kubectl, you can use the tekton CLI to run these quickly without having to navigate the Tekton UI or delete and re-create the resources. The most common command to utilize the existing configs is to instantiate pipeline runs.

To run a tekton pipeline with a [pod template](https://tekton.dev/docs/pipelines/podtemplates/) for Task pods, take the following example:
- `tkn pipeline start create-kit-pipeline -n tekton-tests --pod-template pod-template.yaml -s tekton`
