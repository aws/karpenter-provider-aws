# karpenter

A Helm chart for Karpenter, an open-source node provisioning project built for Kubernetes.

![Version: 0.16.3](https://img.shields.io/badge/Version-0.16.3-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.16.3](https://img.shields.io/badge/AppVersion-0.16.3-informational?style=flat-square)

## Documentation

For full Karpenter documentation please checkout [https://karpenter.sh](https://karpenter.sh/v0.16.3/).

## Installing the Chart

Before the chart can be installed the repo needs to be added to Helm, run the following commands to add the repo.

```bash
helm repo add karpenter https://charts.karpenter.sh/
helm repo update
```

You can follow the detailed installation instruction in the [documentation](https://karpenter.sh/v0.16.3/getting-started/getting-started-with-eksctl/#install) which covers the Karpenter prerequisites and installation options. The outcome of these instructions should result in something like the following command.

```bash
helm upgrade --install --namespace karpenter --create-namespace \
  karpenter karpenter/karpenter \
  --version 0.16.3 \
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
| controller.batchIdleDuration | string | `"1s"` |  |
| controller.batchMaxDuration | string | `"10s"` |  |
| controller.env | list | `[]` | Additional environment variables for the controller pod. |
| controller.extraVolumeMounts | list | `[]` | Additional volumeMounts for the controller pod. |
| controller.image | string | `"public.ecr.aws/karpenter/controller:v0.16.3@sha256:68db4f092cf9cc83f5ef9e2fbc5407c2cb682e81f64dfaa700a7602ede38b1cf"` | Controller image. |
| controller.logEncoding | string | `""` | Controller log encoding, defaults to the global log encoding |
| controller.logLevel | string | `""` | Controller log level, defaults to the global log level |
| controller.resources | object | `{"limits":{"cpu":1,"memory":"1Gi"},"requests":{"cpu":1,"memory":"1Gi"}}` | Resources for the controller pod. |
| controller.securityContext | object | `{}` | SecurityContext for the controller container. |
| dnsConfig | object | `{}` | Configure DNS Config for the pod |
| dnsPolicy | string | `"Default"` | Configure the DNS Policy for the pod |
| extraVolumes | list | `[]` | Additional volumes for the pod. |
| fullnameOverride | string | `""` | Overrides the chart's computed fullname. |
| hostNetwork | bool | `false` | Bind the pod to the host network. This is required when using a custom CNI. |
| imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for Docker images. |
| imagePullSecrets | list | `[]` | Image pull secrets for Docker images. |
| logEncoding | string | `"console"` | Gloabl log encoding |
| logLevel | string | `"debug"` | Global log level |
| nameOverride | string | `""` | Overrides the chart's name. |
| nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Node selectors to schedule the pod to nodes with labels. |
| podAnnotations | object | `{}` | Additional annotations for the pod. |
| podDisruptionBudget.maxUnavailable | int | `1` |  |
| podDisruptionBudget.name | string | `"karpenter"` |  |
| podLabels | object | `{}` | Additional labels for the pod. |
| podSecurityContext | object | `{"fsGroup":1000}` | SecurityContext for the pod. |
| priorityClassName | string | `"system-cluster-critical"` | PriorityClass name for the pod. |
| replicas | int | `2` | Number of replicas. |
| revisionHistoryLimit | int | `10` | The number of old ReplicaSets to retain to allow rollback. |
| serviceAccount.annotations | object | `{}` | Additional annotations for the ServiceAccount. |
| serviceAccount.create | bool | `true` | Specifies if a ServiceAccount should be created. |
| serviceAccount.name | string | `""` | The name of the ServiceAccount to use. If not set and create is true, a name is generated using the fullname template. |
| serviceMonitor.additionalLabels | object | `{}` | Additional labels for the ServiceMonitor. |
| serviceMonitor.enabled | bool | `false` | Specifies whether a ServiceMonitor should be created. |
| serviceMonitor.endpointConfig | object | `{}` | Endpoint configuration for the ServiceMonitor. |
| strategy | object | `{"rollingUpdate":{"maxUnavailable":1}}` | Strategy for updating the pod. |
| terminationGracePeriodSeconds | string | `nil` | Override the default termination grace period for the pod. |
| tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"}]` | Tolerations to allow the pod to be scheduled to nodes with taints. |
| topologySpreadConstraints | list | `[{"maxSkew":1,"topologyKey":"topology.kubernetes.io/zone","whenUnsatisfiable":"ScheduleAnyway"}]` | topologySpreadConstraints to increase the controller resilience |
| webhook.env | list | `[]` | Additional environment variables for the webhook pod. |
| webhook.extraVolumeMounts | list | `[]` | Additional volumeMounts for the webhook pod. |
| webhook.image | string | `"public.ecr.aws/karpenter/webhook:v0.16.3@sha256:96a2d9b06d6bc5127801f358f74b1cf2d289b423a2e9ba40c573c0b14b17dafa"` | Webhook image. |
| webhook.logEncoding | string | `""` | Webhook log encoding, defaults to the global log encoding |
| webhook.logLevel | string | `""` | Webhook log level, defaults to the global log level |
| webhook.port | int | `8443` | The container port to use for the webhook. |
| webhook.resources | object | `{"limits":{"cpu":"200m","memory":"100Mi"},"requests":{"cpu":"200m","memory":"100Mi"}}` | Resources for the webhook pod. |
| webhook.securityContext | object | `{}` | SecurityContext for the webhook container. |

