# karpenter

A Helm chart for Karpenter, an open-source node provisioning project built for Kubernetes.

![Version: 0.6.1](https://img.shields.io/badge/Version-0.6.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.6.1](https://img.shields.io/badge/AppVersion-0.6.1-informational?style=flat-square)

## Documentation

For full Karpenter documentation please checkout [https://karpenter.sh](https://karpenter.sh/v0.6.1/).

## Installing the Chart

Before the chart can be installed the repo needs to be added to Helm, run the following commands to add the repo.

```bash
helm repo add karpenter https://charts.karpenter.sh/
helm repo update
```

You can follow the detailed installation instruction in the [documentation](https://karpenter.sh/v0.6.1/getting-started/#install) which covers the Karpenter prerequisites and installation options. The outcome of these instructions should result in something like the following command.

```bash
helm upgrade --install --namespace karpenter --create-namespace \
  karpenter karpenter/karpenter \
  --version 0.6.1 \
  --set serviceAccount.annotations.eks\.amazonaws\.com/role-arn=${KARPENTER_IAM_ROLE_ARN}
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
| affinity | object | `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"key":"karpenter.sh/provisioner-name","operator":"DoesNotExist"}]}}}` | Affinity rules for scheduling the pod. |
| aws.defaultInstanceProfile | string | `""` | The default instance profile to use when launching nodes on AWS |
| clusterEndpoint | string | `""` | Cluster endpoint. |
| clusterName | string | `""` | Cluster name. |
| controller.env | list | `[]` | Additional environment variables for the controller pod. |
| controller.image | string | `"public.ecr.aws/karpenter/controller:v0.6.1@sha256:5a0bd78e2f7ada324677e2eb82e53b648593e9de1acf0a8fc84138a1a6be753c"` | Controller image. |
| controller.logLevel | string | `""` | Controller log level, defaults to the global log level |
| controller.resources | object | `{"limits":{"cpu":1,"memory":"1Gi"},"requests":{"cpu":1,"memory":"1Gi"}}` | Resources for the controller pod. |
| controller.securityContext | object | `{}` | SecurityContext for the controller container. |
| fullnameOverride | string | `""` | Overrides the chart's computed fullname. |
| hostNetwork | bool | `false` | Bind the pod to the host network. This is required when using a custom CNI. |
| imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for Docker images. |
| imagePullSecrets | list | `[]` | Image pull secrets for Docker images. |
| logLevel | string | `"info"` | Global log level |
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
| webhook.image | string | `"public.ecr.aws/karpenter/webhook:v0.6.1@sha256:7d75747caeb1ca63da1d68925b961c7a61f40faa76aa678320b2d3e090d1713f"` | Webhook image. |
| webhook.logLevel | string | `""` | Webhook log level, defaults to the global log level |
| webhook.port | int | `8443` | The container port to use for the webhook. |
| webhook.resources | object | `{"limits":{"cpu":"100m","memory":"50Mi"},"requests":{"cpu":"100m","memory":"50Mi"}}` | Resources for the webhook pod. |
| webhook.securityContext | object | `{}` | SecurityContext for the webhook container. |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.7.0](https://github.com/norwoodj/helm-docs/releases/v1.7.0)
