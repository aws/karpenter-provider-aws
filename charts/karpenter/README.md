# karpenter

A Helm chart for Karpenter, an open-source node provisioning project built for Kubernetes.

![Version: 1.6.3](https://img.shields.io/badge/Version-1.6.3-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.6.3](https://img.shields.io/badge/AppVersion-1.6.3-informational?style=flat-square)

## Documentation

For full Karpenter documentation please checkout [https://karpenter.sh](https://karpenter.sh/docs/).

## Installing the Chart

You can follow the detailed installation instruction in the [documentation](https://karpenter.sh/docs/getting-started/getting-started-with-karpenter) which covers the Karpenter prerequisites and installation options. The outcome of these instructions should result in something like the following command.

```bash
helm upgrade --install --namespace karpenter --create-namespace \
  karpenter oci://public.ecr.aws/karpenter/karpenter \
  --version 1.6.3 \
  --set "serviceAccount.annotations.eks\.amazonaws\.com/role-arn=${KARPENTER_IAM_ROLE_ARN}" \
  --set settings.clusterName=${CLUSTER_NAME} \
  --set settings.interruptionQueue=${CLUSTER_NAME} \
  --wait
```

### Verification

As the OCI Helm chart is signed by [Cosign](https://github.com/sigstore/cosign) as part of the release process you can verify the chart before installing it by running the following command.

```shell
cosign verify public.ecr.aws/karpenter/karpenter:1.6.3 \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  --certificate-identity-regexp='https://github\.com/aws/karpenter-provider-aws/\.github/workflows/release\.yaml@.+' \
  --certificate-github-workflow-repository=aws/karpenter-provider-aws \
  --certificate-github-workflow-name=Release \
  --certificate-github-workflow-ref=refs/tags/v1.6.3 \
  --annotations version=1.6.3
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| additionalAnnotations | object | `{}` | Additional annotations to add into metadata. |
| additionalClusterRoleRules | list | `[]` | Specifies additional rules for the core ClusterRole. |
| additionalLabels | object | `{}` | Additional labels to add into metadata. |
| affinity | object | `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"karpenter.sh/nodepool","operator":"DoesNotExist"}]}]}},"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"topologyKey":"kubernetes.io/hostname"}]}}` | Affinity rules for scheduling the pod. If an explicit label selector is not provided for pod affinity or pod anti-affinity one will be created from the pod selector labels. |
| controller.containerName | string | `"controller"` | Distinguishing container name (containerName: karpenter-controller). |
| controller.env | list | `[]` | Additional environment variables for the controller pod. |
| controller.envFrom | list | `[]` |  |
| controller.extraVolumeMounts | list | `[]` | Additional volumeMounts for the controller container. |
| controller.healthProbe.port | int | `8081` | The container port to use for http health probe. |
| controller.image.digest | string | `"sha256:b76551596698381c4940c15093afc733357b9ffffd5c97612d324b7a794aa0ed"` | SHA256 digest of the controller image. |
| controller.image.repository | string | `"public.ecr.aws/karpenter/controller"` | Repository path to the controller image. |
| controller.image.tag | string | `"1.6.3"` | Tag of the controller image. |
| controller.metrics.port | int | `8080` | The container port to use for metrics. |
| controller.resources | object | `{}` | Resources for the controller container. |
| controller.securityContext.appArmorProfile | object | `{}` | AppArmor profile for the controller container. |
| controller.securityContext.seLinuxOptions | object | `{}` | SELinux options for the controller container. |
| controller.securityContext.seccompProfile | object | `{}` | Seccomp profile for the controller container. |
| controller.sidecarContainer | list | `[]` | Additional sidecarContainer config |
| controller.sidecarVolumeMounts | list | `[]` | Additional volumeMounts for the sidecar - this will be added to the volume mounts on top of extraVolumeMounts |
| dnsConfig | object | `{}` | Configure DNS Config for the pod |
| dnsPolicy | string | `"ClusterFirst"` | Configure the DNS Policy for the pod |
| extraVolumes | list | `[]` | Additional volumes for the pod. |
| fullnameOverride | string | `""` | Overrides the chart's computed fullname. |
| hostNetwork | bool | `false` | Bind the pod to the host network. This is required when using a custom CNI. |
| imagePullPolicy | string | `"IfNotPresent"` | Image pull policy for Docker images. |
| imagePullSecrets | list | `[]` | Image pull secrets for Docker images. |
| initContainers | object | `{}` | add additional initContainers to run before karpenter container starts |
| logErrorOutputPaths | list | `["stderr"]` | Log errorOutputPaths - defaults to stderr only |
| logLevel | string | `"info"` | Global log level, defaults to 'info' |
| logOutputPaths | list | `["stdout"]` | Log outputPaths - defaults to stdout only |
| nameOverride | string | `""` | Overrides the chart's name. |
| nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Node selectors to schedule the pod to nodes with labels. |
| podAnnotations | object | `{}` | Additional annotations for the pod. |
| podDisruptionBudget.maxUnavailable | int | `1` |  |
| podDisruptionBudget.name | string | `"karpenter"` |  |
| podLabels | object | `{}` | Additional labels for the pod. |
| podSecurityContext | object | `{"fsGroup":65532,"seccompProfile":{"type":"RuntimeDefault"}}` | SecurityContext for the pod. |
| priorityClassName | string | `"system-cluster-critical"` | PriorityClass name for the pod. |
| replicas | int | `2` | Number of replicas. |
| revisionHistoryLimit | int | `10` | The number of old ReplicaSets to retain to allow rollback. |
| schedulerName | string | `"default-scheduler"` | Specify which Kubernetes scheduler should dispatch the pod. |
| service.annotations | object | `{}` | Additional annotations for the Service. |
| serviceAccount.annotations | object | `{}` | Additional annotations for the ServiceAccount. |
| serviceAccount.create | bool | `true` | Specifies if a ServiceAccount should be created. |
| serviceAccount.name | string | `""` | The name of the ServiceAccount to use. If not set and create is true, a name is generated using the fullname template. |
| serviceMonitor.additionalLabels | object | `{}` | Additional labels for the ServiceMonitor. |
| serviceMonitor.enabled | bool | `false` | Specifies whether a ServiceMonitor should be created. |
| serviceMonitor.endpointConfig | object | `{}` | Configuration on `http-metrics` endpoint for the ServiceMonitor. Not to be used to add additional endpoints. See the Prometheus operator documentation for configurable fields https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api-reference/api.md#endpoint |
| serviceMonitor.metricRelabelings | list | `[]` | Metric relabelings for the `http-metrics` endpoint on the ServiceMonitor. For more details on metric relabelings, see: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#metric_relabel_configs |
| serviceMonitor.relabelings | list | `[]` | Relabelings for the `http-metrics` endpoint on the ServiceMonitor. For more details on relabelings, see: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config |
| settings | object | `{"batchIdleDuration":"1s","batchMaxDuration":"10s","clusterCABundle":"","clusterEndpoint":"","clusterName":"","disableDryRun":false,"eksControlPlane":false,"featureGates":{"nodeRepair":false,"reservedCapacity":true,"spotToSpotConsolidation":false},"interruptionQueue":"","isolatedVPC":false,"minValuesPolicy":"Strict","preferencePolicy":"Respect","reservedENIs":"0","vmMemoryOverheadPercent":0.075}` | Global Settings to configure Karpenter |
| settings.batchIdleDuration | string | `"1s"` | The maximum amount of time with no new ending pods that if exceeded ends the current batching window. If pods arrive faster than this time, the batching window will be extended up to the maxDuration. If they arrive slower, the pods will be batched separately. |
| settings.batchMaxDuration | string | `"10s"` | The maximum length of a batch window. The longer this is, the more pods we can consider for provisioning at one time which usually results in fewer but larger nodes. |
| settings.clusterCABundle | string | `""` | Cluster CA bundle for TLS configuration of provisioned nodes. If not set, this is taken from the controller's TLS configuration for the API server. |
| settings.clusterEndpoint | string | `""` | Cluster endpoint. If not set, will be discovered during startup (EKS only). |
| settings.clusterName | string | `""` | Cluster name. |
| settings.disableDryRun | bool | `false` | Disable dry run validation for EC2NodeClasses. |
| settings.eksControlPlane | bool | `false` | Marking this true means that your cluster is running with an EKS control plane and Karpenter should attempt to discover cluster details from the DescribeCluster API. |
| settings.featureGates | object | `{"nodeRepair":false,"reservedCapacity":true,"spotToSpotConsolidation":false}` | Feature Gate configuration values. Feature Gates will follow the same graduation process and requirements as feature gates in Kubernetes. More information here https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-gates-for-alpha-or-beta-features. |
| settings.featureGates.nodeRepair | bool | `false` | nodeRepair is ALPHA and is disabled by default. Setting this to true will enable node repair. |
| settings.featureGates.reservedCapacity | bool | `true` | reservedCapacity is BETA and is enabled by default. Setting this will enable native on-demand capacity reservation support. |
| settings.featureGates.spotToSpotConsolidation | bool | `false` | spotToSpotConsolidation is ALPHA and is disabled by default. Setting this to true will enable spot replacement consolidation for both single and multi-node consolidation. |
| settings.interruptionQueue | string | `""` | Interruption queue is the name of the SQS queue used for processing interruption events from EC2. Interruption handling is disabled if not specified. Enabling interruption handling may require additional permissions on the controller service account. Additional permissions are outlined in the docs. |
| settings.isolatedVPC | bool | `false` | If true then assume we can't reach AWS services which don't have a VPC endpoint. This also has the effect of disabling look-ups to the AWS pricing endpoint. |
| settings.minValuesPolicy | string | `"Strict"` | How the Karpenter scheduler treats min values. Options include 'Strict' (fails scheduling when min values can't be met) and 'BestEffort' (relaxes min values when they can't be met). |
| settings.preferencePolicy | string | `"Respect"` | How the Karpenter scheduler should treat preferences. Preferences include preferredDuringSchedulingIgnoreDuringExecution node and pod affinities/anti-affinities and ScheduleAnyways topologySpreadConstraints. Can be one of 'Ignore' and 'Respect' |
| settings.reservedENIs | string | `"0"` | Reserved ENIs are not included in the calculations for max-pods or kube-reserved. This is most often used in the VPC CNI custom networking setup https://docs.aws.amazon.com/eks/latest/userguide/cni-custom-network.html. |
| settings.vmMemoryOverheadPercent | float | `0.075` | The VM memory overhead as a percent that will be subtracted from the total memory for all instance types. The value of `0.075` equals to 7.5%. |
| strategy | object | `{"rollingUpdate":{"maxUnavailable":1}}` | Strategy for updating the pod. |
| terminationGracePeriodSeconds | string | `nil` | Override the default termination grace period for the pod. |
| tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"}]` | Tolerations to allow the pod to be scheduled to nodes with taints. |
| topologySpreadConstraints | list | `[{"maxSkew":1,"topologyKey":"topology.kubernetes.io/zone","whenUnsatisfiable":"DoNotSchedule"}]` | Topology spread constraints to increase the controller resilience by distributing pods across the cluster zones. If an explicit label selector is not provided one will be created from the pod selector labels. |

----------------------------------------------

Autogenerated from chart metadata using [helm-docs](https://github.com/norwoodj/helm-docs/).
