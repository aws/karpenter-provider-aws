---
title: "Using Nitro Enclaves"
linkTitle: "Using Nitro Enclaves"
weight: 20
description: >
  Configure AWS Nitro Enclaves with EC2NodeClasses and NodeOverlays
---

This guide shows you how to configure Karpenter to provision nodes with AWS Nitro Enclaves enabled.

Nitro Enclaves provide an isolated compute environment to protect and process highly sensitive data such as personally identifiable information (PII), healthcare, financial, and intellectual property data.

## What are Nitro Enclaves?

AWS Nitro Enclaves are isolated compute environments built on the AWS Nitro System that provide:

- **Isolated compute environment**: Enclaves run in a separate memory space from the parent instance with no persistent storage, interactive access, or external networking
- **Cryptographic attestation**: You can verify the enclave's identity and integrity before trusting it with sensitive data
- **Reduced attack surface**: No SSH access, no persistent storage, and limited communication to only the parent instance
- **CPU and memory isolation**: Dedicated vCPUs and memory allocated from the parent instance

Common use cases include:

- Processing sensitive financial data and transactions
- Running confidential computing workloads
- Implementing secure key management and cryptographic operations
- Supporting compliance efforts for workloads with data-isolation requirements, such as PCI DSS or HIPAA-regulated workloads

For more information, see the [AWS Nitro Enclaves documentation](https://docs.aws.amazon.com/enclaves/).

Karpenter enables Nitro Enclaves in EC2 launch templates, excludes instance types whose EC2 `NitroEnclavesSupport` value is not `supported`, and models `NodeOverlay` capacity. You must configure the node, install the Kubernetes device plugin, and keep the `NodeOverlay` capacity consistent with the resources advertised by the device plugin and kubelet.

{{% alert title="Warning" color="warning" %}}
Karpenter does not inspect your AMI, generate Nitro Enclaves bootstrap configuration, or subtract enclave CPU from the instance's standard allocatable `cpu`. Hugepage resources declared in a `NodeOverlay` are subtracted from standard allocatable `memory` in Karpenter's scheduling simulation. The actual allocation happens on the node; a `NodeOverlay` does not configure the node or modify the capacity reported by kubelet.
{{% /alert %}}

{{% alert title="Warning" color="warning" %}}
All pods and containers on the parent node can communicate with enclaves attached to that node. A taint controls scheduling; it is not a security boundary. Use a dedicated NodePool with a `NoSchedule` taint to reduce which workloads can run on these nodes, and add the matching toleration only to the device-plugin DaemonSet and enclave workloads.
{{% /alert %}}

## Prerequisites

Before continuing:

1. Enable the alpha `NodeOverlay` feature gate.
2. Prepare an AMI or user data that [configures Nitro Enclaves on the node](https://docs.aws.amazon.com/enclaves/latest/user/kubernetes.html), including the allocator and the 1 GiB hugepages used by this example.
3. Install the [AWS Nitro Enclaves Kubernetes device plugin](https://github.com/aws/aws-nitro-enclaves-k8s-device-plugin) on nodes labeled `aws-nitro-enclaves-k8s-dp: enabled`. Configure the device-plugin DaemonSet as shown below.
4. Decide how many enclave slots and CPUs and how much memory each node will advertise.

## Enable NodeOverlays

When installing Karpenter with Helm, enable NodeOverlays through the chart settings:

```yaml
settings:
  featureGates:
    nodeOverlay: true
```

## Enable Nitro Enclaves on the EC2NodeClass

Set `spec.enclaveOptions.enabled` to `true`:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: enclave-enabled
spec:
  enclaveOptions:
    enabled: true

  amiFamily: AL2023
  amiSelectorTerms:
    - alias: al2023@latest

  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
  role: "KarpenterNodeRole-${CLUSTER_NAME}"
```

Switching `enabled` between `true` and `false` changes the generated launch template and participates in EC2NodeClass drift. When `enabled` is `true`, Karpenter also filters the EC2 instance-type candidates to those whose `NitroEnclavesSupport` value is `supported`.

The example uses an EKS-optimized AMI only to keep the EC2NodeClass concise. You must still provide the node-level Nitro Enclaves configuration, either by baking it into an AMI or supplying appropriate user data. It uses `@latest` for brevity; follow the [AMI pinning guidance]({{< relref "managing-amis#pinning-amis" >}}) for production.

For compatibility with existing configurations, omitting `enclaveOptions` disables Nitro Enclaves unless a NodeClaim requests `eks.amazonaws.com/nitro-sandbox`. When `enclaveOptions` is specified, `enabled` is required. Setting `enabled` to `false` explicitly disables Nitro Enclaves; a conflicting NodeClaim fails before EC2 instance creation with reason `NitroEnclavesDisabled`. Karpenter retries the NodeClaim until its launch timeout, and the workload remains pending until the conflict is resolved.

{{% alert title="Warning" color="warning" %}}
[Nitro Enclaves are not supported in AWS Local Zones, AWS Wavelength Zones, or AWS Outposts](https://docs.aws.amazon.com/enclaves/latest/user/nitro-enclave.html). Configure `subnetSelectorTerms` to resolve only subnets in standard Availability Zones. Karpenter filters instance types based on EC2's `NitroEnclavesSupport` value, but does not filter these unsupported locations. EC2 can reject launch attempts that target them. If only unsupported locations match, the workload remains pending.
{{% /alert %}}

## Label the NodePool

Use the default device-plugin label on the nodes that should run the plugin:

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: enclave-pool
spec:
  template:
    metadata:
      labels:
        aws-nitro-enclaves-k8s-dp: enabled
    spec:
      taints:
        - key: aws-nitro-enclaves-k8s-dp
          value: enabled
          effect: NoSchedule
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: enclave-enabled
```

You do not need to maintain a static list of enclave-capable instance families. The EC2NodeClass compatibility filter removes unsupported instance types. You can add normal NodePool requirements when you need tighter cost, architecture, or size constraints.

## Configure the Device Plugin

The upstream device-plugin DaemonSet selects nodes with the `aws-nitro-enclaves-k8s-dp: enabled` label. Enable enclave CPU advertisement and add a toleration for the NodePool taint:

```bash
kubectl patch daemonset aws-nitro-enclaves-k8s-daemonset \
  --namespace kube-system \
  --type strategic \
  --patch '{
    "spec": {
      "template": {
        "spec": {
          "tolerations": [{
            "key": "aws-nitro-enclaves-k8s-dp",
            "operator": "Equal",
            "value": "enabled",
            "effect": "NoSchedule"
          }],
          "containers": [{
            "name": "aws-nitro-enclaves-k8s-dp",
            "env": [{
              "name": "ENCLAVE_CPU_ADVERTISEMENT",
              "value": "true"
            }]
          }]
        }
      }
    }
  }'
```

Without the toleration, the DaemonSet cannot run on the tainted nodes. Without CPU advertisement, the device plugin does not publish the `aws.ec2.nitro/nitro_enclaves_cpus` resource used by the example.

## Model Enclave Resources with a NodeOverlay

Create a `NodeOverlay` that matches the device-plugin label. Include numeric CPU and memory requirements so the overlay is only considered for instances larger than the enclave reservation.

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: nitro-enclaves
spec:
  requirements:
    - key: aws-nitro-enclaves-k8s-dp
      operator: In
      values: ["enabled"]
    - key: karpenter.k8s.aws/instance-cpu
      operator: Gt
      values: ["2"]
    - key: karpenter.k8s.aws/instance-memory
      operator: Gt
      values: ["4096"]
  capacity:
    aws.ec2.nitro/nitro_enclaves: "4"
    aws.ec2.nitro/nitro_enclaves_cpus: "2"
    hugepages-1Gi: "4Gi"
```

The `instance-memory` requirement is expressed in MiB. Adjust the capacity values to match the device plugin, allocator, and hugepage configuration on your nodes.

The overlay must describe what the device plugin and kubelet will actually advertise:

- `aws.ec2.nitro/nitro_enclaves` represents the number of enclaves
- `aws.ec2.nitro/nitro_enclaves_cpus` represents enclave vCPUs
- `hugepages-1Gi` represents memory reserved as 1 GiB hugepages

## Schedule an Enclave Workload

Request the same resources from the workload:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: enclave-workload
spec:
  tolerations:
    - key: aws-nitro-enclaves-k8s-dp
      operator: Equal
      value: enabled
      effect: NoSchedule
  containers:
    - name: workload
      image: public.ecr.aws/your-repository/your-image:latest
      volumeMounts:
        - name: hugepages
          mountPath: /dev/hugepages
      resources:
        requests:
          cpu: 250m
          aws.ec2.nitro/nitro_enclaves: "1"
          aws.ec2.nitro/nitro_enclaves_cpus: "2"
          hugepages-1Gi: "4Gi"
        limits:
          aws.ec2.nitro/nitro_enclaves: "1"
          aws.ec2.nitro/nitro_enclaves_cpus: "2"
          hugepages-1Gi: "4Gi"
  volumes:
    - name: hugepages
      emptyDir:
        medium: HugePages-1Gi
```

Replace the placeholder image with an enclave application that starts an enclave. The workload does not need to select a NodePool or instance family. Karpenter matches these resource requests against the capacity declared by the `NodeOverlay`.

## Validate the Configuration

Confirm that the `NodeOverlay` is ready and passed validation:

```bash
kubectl get nodeoverlay nitro-enclaves \
  -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}'
```

Verify that the output includes `Ready=True` and `ValidationSucceeded=True`.

Confirm that EC2 launched the instance with enclaves enabled:

```bash
INSTANCE_ID="i-0123456789abcdef0"
aws ec2 describe-instances \
  --instance-ids "${INSTANCE_ID}" \
  --query 'Reservations[].Instances[].EnclaveOptions'
```

Verify that `Enabled` is `true`.

On the node, use the Nitro CLI to inspect running enclaves:

```bash
nitro-cli describe-enclaves
```

Confirm that the real node capacity matches the overlay:

```bash
NODE_NAME="ip-10-0-1-2.us-west-2.compute.internal"
kubectl get node "${NODE_NAME}" -o json | jq '.status.capacity | {
  enclaves: ."aws.ec2.nitro/nitro_enclaves",
  enclaveCPUs: ."aws.ec2.nitro/nitro_enclaves_cpus",
  hugepages: ."hugepages-1Gi"
}'
```

If these values are absent or differ from the `NodeOverlay`, fix the node image, allocator, hugepage configuration, or device-plugin deployment. Changing only the overlay can make Karpenter provision a node for a pod that the Kubernetes scheduler cannot place.

Confirm that the enclave workload schedules successfully:

```bash
kubectl get pod enclave-workload -o wide
```

## Capacity Considerations

The enclave allocator takes CPU and memory from the parent instance. The `hugepages-1Gi` capacity in this example reduces the allocatable memory Karpenter computes, but `aws.ec2.nitro/nitro_enclaves_cpus` does not reduce the allocatable CPU Karpenter computes.

After launch, compare standard `cpu` in the NodeClaim's `.status.allocatable` with the Node's `.status.allocatable`. Keep ordinary workloads off the enclave NodePool. The sum of standard `cpu` requests for every pod allowed onto the NodePool must fit the Node's actual allocatable CPU. Separately, the sum of `aws.ec2.nitro/nitro_enclaves_cpus` requests must fit the capacity declared by the `NodeOverlay`. Requesting enclave CPUs does not reserve standard CPU, and a `NoSchedule` taint does not change Karpenter's CPU calculation.

## Additional Resources

- [AWS Nitro Enclaves documentation](https://docs.aws.amazon.com/enclaves/)
- [Nitro Enclaves SDK](https://github.com/aws/aws-nitro-enclaves-sdk-c)
- [AWS Nitro Enclaves Kubernetes device plugin](https://github.com/aws/aws-nitro-enclaves-k8s-device-plugin)
- [EC2NodeClass reference]({{< relref "../concepts/nodeclasses#specenclaveoptions" >}})
- [NodeOverlay reference]({{< relref "../concepts/nodeoverlays" >}})
- [NodePool reference]({{< relref "../concepts/nodepools" >}})
- [Karpenter disruption documentation]({{< relref "../concepts/disruption" >}})

## Follow-up

If you have questions or issues with Nitro Enclaves in Karpenter, feel free to:

- Open an issue on [GitHub](https://github.com/aws/karpenter-provider-aws/issues/new/choose)
- Ask in the [Karpenter Slack channel](https://kubernetes.slack.com/archives/C02SFFZSA2K)
- Check the [Troubleshooting Guide]({{< ref "../troubleshooting" >}})
