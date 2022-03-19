# karpenter

A Helm chart for Karpenter, an open-source node provisioning project built for Kubernetes.

![Version: 0.7.2](https://img.shields.io/badge/Version-0.7.2-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.7.2](https://img.shields.io/badge/AppVersion-0.7.2-informational?style=flat-square)

## Documentation

For full Karpenter documentation please checkout [https://karpenter.sh](https://karpenter.sh/v0.7.2/).

## Installing the Chart

Before the chart can be installed the repo needs to be added to Helm, run the following commands to add the repo.

```bash
helm repo add karpenter https://charts.karpenter.sh/
helm repo update
```

You can follow the detailed installation instruction in the [documentation](https://karpenter.sh/v0.7.2/getting-started/getting-started-with-eksctl/#install) which covers the Karpenter prerequisites and installation options. The outcome of these instructions should result in something like the following command.

```bash
helm upgrade --install --namespace karpenter --create-namespace \
  karpenter karpenter/karpenter \
  --version 0.7.2 \
  --set serviceAccount.annotations.eks\.amazonaws\.com/role-arn=${KARPENTER_IAM_ROLE_ARN} \
  --set clusterName=${CLUSTER_NAME} \
  --set clusterEndpoint=${CLUSTER_ENDPOINT} \
  --set aws.defaultInstanceProfile=KarpenterNodeInstanceProfile-${CLUSTER_NAME} \
  --wait # for the defaulting webhook to install before creating a Provisioner
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| additionalAnnotations | object | `{}` | Additional annotations to add into metadata. |
| additionalLabels | object | `{}` | Additional labels to add into metadata. |
| affinity | object | `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"karpenter.sh/provisioner-name","operator":"DoesNotExist"}]}]}}}` | Affinity rules for scheduling the pod. |
| aws.defaultInstanceProfile | string | `""` | The default instance profile to use when launching nodes on AWS |
| clusterEndpoint | string | `""` | Cluster endpoint. |
| clusterName | string | `""` | Cluster name. |
| controller.env | list | `[]` | Additional environment variables for the controller pod. |
| controller.image | string | `"public.ecr.aws/karpenter/controller:v0.7.2@sha256:66f83fd6ad026be1fb99a8b357bfa2b6656e3ddfd3423302d6d7b7d892954da1"` | Controller image. |
| controller.logLevel | string | `""` | Controller log level, defaults to the global log level |
| controller.resources | object | `{"limits":{"cpu":1,"memory":"1Gi"},"requests":{"cpu":1,"memory":"1Gi"}}` | Resources for the controller pod. |
| controller.securityContext | object | `{}` | SecurityContext for the controller container. |
| fullnameOverride | string | `""` | Overrides the chart's computed fullname. |
| hostNetwork | bool | `false` | Bind the pod to the host network. This is required when using a custom CNI. |
| imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for Docker images. |
| imagePullSecrets | list | `[]` | Image pull secrets for Docker images. |
| logLevel | string | `"debug"` | Global log level |
| nameOverride | string | `""` | Overrides the chart's name. |
| nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Node selectors to schedule the pod to nodes with labels. |
| podAnnotations | object | `{}` | Additional annotations for the pod. |
| podLabels | object | `{}` | Additional labels for the pod. |
| podSecurityContext | object | `{"fsGroup":1000}` | SecurityContext for the pod. |
| priorityClassName | string | `"system-cluster-critical"` | PriorityClass name for the pod. |
| replicas | int | `1` | Number of replicas. |
| serviceAccount.annotations | object | `{}` | Additional annotations for the ServiceAccount. |
| serviceAccount.create | bool | `true` | Specifies if a ServiceAccount should be created. |
| serviceAccount.name | string | `""` | The name of the ServiceAccount to use. If not set and create is true, a name is generated using the fullname template. |
| serviceMonitor.additionalLabels | object | `{}` | Additional labels for the ServiceMonitor. |
| serviceMonitor.enabled | bool | `false` | Specifies whether a ServiceMonitor should be created. |
| serviceMonitor.endpointConfig | object | `{}` | Endpoint configuration for the ServiceMonitor. |
| strategy | object | `{"type":"Recreate"}` | Strategy for updating the pod. |
| terminationGracePeriodSeconds | string | `nil` | Override the default termination grace period for the pod. |
| tolerations | list | `[]` | Tolerations to allow the pod to be scheduled to nodes with taints. |
| webhook.env | list | `[]` | Additional environment variables for the webhook pod. |
| webhook.image | string | `"public.ecr.aws/karpenter/webhook:v0.7.2@sha256:86595b8374c572d49aeeec033fbcf320dc46f7b7bd01ff1af65163168f90f2ad"` | Webhook image. |
| webhook.logLevel | string | `""` | Webhook log level, defaults to the global log level |
| webhook.port | int | `8443` | The container port to use for the webhook. |
| webhook.resources | object | `{"limits":{"cpu":"100m","memory":"50Mi"},"requests":{"cpu":"100m","memory":"50Mi"}}` | Resources for the webhook pod. |
| webhook.securityContext | object | `{}` | SecurityContext for the webhook container. |

