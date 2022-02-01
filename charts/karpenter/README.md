# karpenter

A Helm chart for https://github.com/aws/karpenter/.

![Version: 0.6.0](https://img.shields.io/badge/Version-0.6.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.6.0](https://img.shields.io/badge/AppVersion-0.6.0-informational?style=flat-square)

## Installing the Chart

To install the chart with the release name `karpenter`:

```console
$ helm repo add karpenter https://charts.karpenter.sh
$ helm repo update
$ helm upgrade --install karpenter karpenter/karpenter --namespace karpenter \
  --create-namespace --set serviceAccount.create=false --version 0.6.0 \
  --set controller.clusterName=${CLUSTER_NAME} \
  --set controller.clusterEndpoint=$(aws eks describe-cluster --name ${CLUSTER_NAME} --query "cluster.endpoint" --output json) \
  --wait # for the defaulting webhook to install before creating a Provisioner 
```

You can follow the detailed installation instruction [here](https://karpenter.sh/docs/getting-started/#install).

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| additionalLabels | object | `{}` | Additional labels to add into metadata |
| aws.defaultInstanceProfile | string | `""` | The default instance profile to use when launching nodes on AWS |
| controller.affinity | object | `{}` | Affinity rules for scheduling |
| controller.clusterEndpoint | string | `""` | Cluster endpoint |
| controller.clusterName | string | `""` | Cluster name |
| controller.env | list | `[]` | Additional environment variables to run with |
| controller.image | string | `"public.ecr.aws/karpenter/controller:v0.6.0@sha256:c4b55bafc91bcab268c7c80c98f4341fc23ab0adc29ba33e28a1f9df1ec96de5"` | Image to use for the Karpenter controller |
| controller.nodeSelector | object | `{}` | Node selectors to schedule to nodes with labels. |
| controller.replicas | int | `1` |  |
| controller.resources.limits.cpu | int | `1` |  |
| controller.resources.limits.memory | string | `"1Gi"` |  |
| controller.resources.requests.cpu | int | `1` |  |
| controller.resources.requests.memory | string | `"1Gi"` |  |
| controller.tolerations | list | `[]` | Tolerations to schedule to nodes with taints. |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account (like the ARN of the IRSA role) |
| serviceAccount.create | bool | `true` | Create a service account for the application controller |
| serviceAccount.name | string | `"karpenter"` | Service account name |
| webhook.affinity | object | `{}` | Affinity rules for scheduling |
| webhook.env | list | `[]` | List of environment items to add to the webhook |
| webhook.hostNetwork | bool | `false` | Set to true if using custom CNI on EKS |
| webhook.image | string | `"public.ecr.aws/karpenter/webhook:v0.6.0@sha256:bce76e56b8315c7f5ebe097a738ef81e9a07f84cfdc5da1e55975ba17783d0dc"` | Image to use for the webhook |
| webhook.nodeSelector | object | `{}` | Node selectors to schedule to nodes with labels. |
| webhook.port | int | `8443` |  |
| webhook.replicas | int | `1` |  |
| webhook.resources.limits.cpu | string | `"100m"` |  |
| webhook.resources.limits.memory | string | `"50Mi"` |  |
| webhook.resources.requests.cpu | string | `"100m"` |  |
| webhook.resources.requests.memory | string | `"50Mi"` |  |
| webhook.tolerations | list | `[]` | Tolerations to schedule to nodes with taints. |

