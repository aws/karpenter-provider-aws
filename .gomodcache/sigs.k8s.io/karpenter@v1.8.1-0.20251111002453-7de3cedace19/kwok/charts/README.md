# karpenter

A Helm chart for Karpenter's KwoK Provider, integrated with https://github.com/kubernetes-sigs/kwok.

![Version: 0.35.0](https://img.shields.io/badge/Version-0.35.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.35.0](https://img.shields.io/badge/AppVersion-0.35.0-informational?style=flat-square)

## Documentation

For full Karpenter documentation please checkout [https://karpenter.sh](https://karpenter.sh/docs/).

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| additionalAnnotations | object | `{}` | Additional annotations to add into metadata. |
| additionalClusterRoleRules | list | `[]` | Specifies additional rules for the core ClusterRole. |
| additionalLabels | object | `{}` | Additional labels to add into metadata. |
| affinity | object | `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"karpenter.sh/nodepool","operator":"DoesNotExist"}]}]}},"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"topologyKey":"kubernetes.io/hostname"}]}}` | Affinity rules for scheduling the pod. If an explicit label selector is not provided for pod affinity or pod anti-affinity one will be created from the pod selector labels. |
| controller.env | list | `[]` | Additional environment variables for the controller pod. |
| controller.envFrom | list | `[]` |  |
| controller.extraVolumeMounts | list | `[]` | Additional volumeMounts for the controller pod. |
| controller.healthProbe.port | int | `8081` | The container port to use for http health probe. |
| controller.image.digest | string | `""` | SHA256 digest of the controller image. |
| controller.image.repository | string | `""` | Repository path to the controller image. |
| controller.image.tag | string | `""` | Tag of the controller image. |
| controller.metrics.port | int | `8080` | The container port to use for metrics. |
| controller.resources | object | `{}` | Resources for the controller pod. |
| controller.sidecarContainer | list | `[]` | Additional sidecarContainer config |
| controller.sidecarVolumeMounts | list | `[]` | Additional volumeMounts for the sidecar - this will be added to the volume mounts on top of extraVolumeMounts |
| dnsConfig | object | `{}` | Configure DNS Config for the pod |
| dnsPolicy | string | `"Default"` | Configure the DNS Policy for the pod |
| extraVolumes | list | `[]` | Additional volumes for the pod. |
| fullnameOverride | string | `""` | Overrides the chart's computed fullname. |
| hostNetwork | bool | `false` | Bind the pod to the host network. This is required when using a custom CNI. |
| imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for Docker images. |
| imagePullSecrets | list | `[]` | Image pull secrets for Docker images. |
| logLevel | string | `"info"` | Global log level, defaults to 'info' |
| nameOverride | string | `""` | Overrides the chart's name. |
| nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Node selectors to schedule the pod to nodes with labels. |
| podAnnotations | object | `{}` | Additional annotations for the pod. |
| podDisruptionBudget.maxUnavailable | int | `1` |  |
| podDisruptionBudget.name | string | `"karpenter"` |  |
| podLabels | object | `{}` | Additional labels for the pod. |
| podSecurityContext | object | `{"fsGroup":65536}` | SecurityContext for the pod. |
| priorityClassName | string | `"system-cluster-critical"` | PriorityClass name for the pod. |
| replicas | int | `1` | Number of replicas. |
| revisionHistoryLimit | int | `10` | The number of old ReplicaSets to retain to allow rollback. |
| serviceAccount.annotations | object | `{}` | Additional annotations for the ServiceAccount. |
| serviceAccount.create | bool | `true` | Specifies if a ServiceAccount should be created. |
| serviceAccount.name | string | `""` | The name of the ServiceAccount to use. If not set and create is true, a name is generated using the fullname template. |
| serviceMonitor.additionalLabels | object | `{}` | Additional labels for the ServiceMonitor. |
| serviceMonitor.enabled | bool | `false` | Specifies whether a ServiceMonitor should be created. |
| serviceMonitor.endpointConfig | object | `{}` | Endpoint configuration for the ServiceMonitor. |
| settings | object | `{"batchIdleDuration":"1s","batchMaxDuration":"10s","featureGates":{"nodeRepair":false,"spotToSpotConsolidation":false},"minValuesPolicy":"Strict","preferencePolicy":"Respect"}` | Global Settings to configure Karpenter |
| settings.batchIdleDuration | string | `"1s"` | The maximum amount of time with no new pending pods that if exceeded ends the current batching window. If pods arrive faster than this time, the batching window will be extended up to the maxDuration. If they arrive slower, the pods will be batched separately. |
| settings.batchMaxDuration | string | `"10s"` | The maximum length of a batch window. The longer this is, the more pods we can consider for provisioning at one time which usually results in fewer but larger nodes. |
| settings.featureGates | object | `{"nodeRepair":false,"spotToSpotConsolidation":false}` | Feature Gate configuration values. Feature Gates will follow the same graduation process and requirements as feature gates in Kubernetes. More information here https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-gates-for-alpha-or-beta-features |
| settings.featureGates.nodeRepair | bool | `false` | nodeRepair is ALPHA and is disabled by default. Setting this to true will enable node repair. |
| settings.featureGates.spotToSpotConsolidation | bool | `false` | spotToSpotConsolidation is ALPHA and is disabled by default. Setting this to true will enable spot replacement consolidation for both single and multi-node consolidation. |
| settings.minValuesPolicy | string | `"Strict"` | How the Karpenter scheduler treats min values. Options include 'Strict' (fails scheduling when min values can't be met) and 'BestEffort' (relaxes min values when they can't be met). |
| settings.preferencePolicy | string | `"Respect"` | How the Karpenter scheduler should treat preferences. Preferences include preferredDuringSchedulingIgnoreDuringExecution node and pod affinities/anti-affinities and ScheduleAnyways topologySpreadConstraints. Can be one of 'Ignore' and 'Respect' |
| strategy | object | `{"rollingUpdate":{"maxUnavailable":1}}` | Strategy for updating the pod. |
| terminationGracePeriodSeconds | string | `nil` | Override the default termination grace period for the pod. |
| tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"}]` | Tolerations to allow the pod to be scheduled to nodes with taints. |
| topologySpreadConstraints | list | `[{"maxSkew":1,"topologyKey":"topology.kubernetes.io/zone","whenUnsatisfiable":"ScheduleAnyway"}]` | Topology spread constraints to increase the controller resilience by distributing pods across the cluster zones. If an explicit label selector is not provided one will be created from the pod selector labels. |

