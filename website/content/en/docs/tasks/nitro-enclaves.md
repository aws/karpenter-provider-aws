---
title: "Using Nitro Enclaves"
linkTitle: "Using Nitro Enclaves"
weight: 20
description: >
  Task for using AWS Nitro Enclaves with Karpenter
---

This guide shows you how to configure Karpenter to provision nodes with AWS Nitro Enclaves enabled. Nitro Enclaves provide an isolated compute environment to protect and process highly sensitive data such as personally identifiable information (PII), healthcare, financial, and intellectual property data.

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
- Meeting compliance requirements that mandate data isolation (PCI-DSS, HIPAA, etc.)

For more information, see the [AWS Nitro Enclaves documentation](https://docs.aws.amazon.com/enclaves/).

## Prerequisites

Before enabling Nitro Enclaves with Karpenter, ensure you have:

1. **Compatible Instance Types**: Not all instance types support Nitro Enclaves. Check the [AWS Nitro Enclave requirements](https://docs.aws.amazon.com/enclaves/latest/user/nitro-enclave.html#nitro-enclave-reqs) for compatible instance types. **Instance types that do not support Nitro Enclaves will fail to launch with enclaves enabled.**

2. **Node Overlay Feature**: You must enable the `NodeOverlay` feature gate in Karpenter and use [Node Overlays](https://karpenter.sh/docs/concepts/nodeoverlays/) to properly configure enclave resources for Karpenter's scheduling simulation. This allows Karpenter to account for CPU and memory allocated to enclaves without requiring Custom AMI configuration.

## Configuration Overview

To use Nitro Enclaves with Karpenter, you need to:

1. Enable the `NodeOverlay` feature gate in your Karpenter deployment
2. Enable enclaves in your `EC2NodeClass` using `spec.enclaveOptions`
3. Configure your `NodePool` to use enclave-compatible instance types
4. Create a `NodeOverlay` resource to adjust node capacity for enclave resource allocation
5. Deploy workloads that request the enclave extended resource

## Step-by-Step Configuration

### Step 1: Enable NodeOverlay Feature Gate

First, enable the `NodeOverlay` feature gate in your Karpenter deployment. Update your Karpenter Helm values or deployment:

```yaml
# values.yaml for Helm installation
controller:
  env:
    - name: FEATURE_GATES
      value: "NodeOverlay=true"
```

Or if deploying directly:

```bash
# Update the Karpenter deployment
kubectl set env deployment/karpenter -n karpenter FEATURE_GATES="NodeOverlay=true"
```

### Step 2: Create an EC2NodeClass with Enclave Options

Create an `EC2NodeClass` with `enclaveOptions.enabled: true`:

```yaml
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: enclave-enabled
spec:
  # Enable Nitro Enclaves
  enclaveOptions:
    enabled: true

  # Select your AMI family
  amiFamily: AL2023
  amiSelectorTerms:
    - alias: al2023@latest

  # Required: Configure subnet and security group selection
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"

  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"

 ...
```

{{% alert title="Important" color="warning" %}}
The `enclaveOptions.enabled: true` setting enables Nitro Enclaves at the EC2 instance level. However, you also need to:
1. Set up the nitro-cli to configure your enclave
2. Ensure the memory configuration matches what you declare in your NodeOverlay
3. The NodeOverlay tells Karpenter's scheduler about the resources, but the actual allocation happens on the node
{{% /alert %}}

### Step 3: Create a NodePool

Create a `NodePool` that references your enclave-enabled `EC2NodeClass`:

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: enclave-pool
spec:
  template:
    spec:
      # Reference the enclave-enabled EC2NodeClass
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: enclave-enabled

      # Constrain to enclave-compatible instance types
      # Note: Not all instance sizes support enclaves (e.g., m5.large does not)
      requirements:
        - key: karpenter.k8s.aws/instance-family
          operator: In
          values:
            - m5
            - m6i
            - c5
            - c6i
            - r5
            - r6i
        - key: karpenter.k8s.aws/instance-size
          operator: In
          values:
            - xlarge
            - 2xlarge
            - 4xlarge

  # Configure disruption settings
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized
    consolidateAfter: 1m
```

{{% alert title="Note" color="primary" %}}
Not all instance sizes within an enclave-compatible instance family support Nitro Enclaves. For example, `m5.large` does not support enclaves, but `m5.xlarge` and larger sizes do. Refer to the [AWS documentation](https://docs.aws.amazon.com/enclaves/latest/user/nitro-enclave.html#nitro-enclave-reqs) for the complete list of supported instance types.
{{% /alert %}}

### Step 4: Create a NodeOverlay for Enclave Resources

Create a `NodeOverlay` resource to add extended resources for enclave support. NodeOverlays can only add extended resources - they cannot modify standard resources like CPU or memory. The NodeOverlay should target your specific NodePool to ensure it only applies to enclave-enabled nodes:

```yaml
apiVersion: karpenter.sh/v1alpha1
kind: NodeOverlay
metadata:
  name: nitro-enclaves-overlay
spec:
  # Target the enclave NodePool specifically
  requirements:
    - key: karpenter.sh/nodepool
      operator: In
      values:
        - enclave-pool

  # Add extended resources for enclave support
  capacity:
    # Mark this node as having enclave capability
    aws.amazon.com/nitro-enclaves: "1"
    # Enclave memory is typically allocated as hugepages
    # This adds 4Gi of 2Mi hugepages that the enclave can use
    hugepages-2Mi: "4Gi"
```

## Validating Your Setup

After deploying your configuration, you can verify that Nitro Enclaves are properly enabled:

### 1. Check Node Resources

Verify that the node has the extended resources from the NodeOverlay:

```bash
kubectl get nodes -o json | jq '.items[] | select(.metadata.labels["karpenter.sh/nodepool"] == "enclave-pool") | {name: .metadata.name, capacity: .status.capacity}'
```

You should see output similar to:

```json
{
  "name": "ip-10-0-1-234.ec2.internal",
  "capacity": {
    "aws.amazon.com/nitro-enclaves": "1",
    "cpu": "4",
    "hugepages-2Mi": "4Gi",
    "memory": "16384Mi",
    ...
  }
}
```

### 2. Verify Enclave Support on the Instance

SSH into the node or use AWS Systems Manager to verify that the instance was launched with enclave support:

```bash
# Check if the Nitro CLI is available and enclaves are supported
nitro-cli describe-enclaves

# Check instance metadata for enclave support
curl -s http://169.254.169.254/latest/meta-data/enclave-options/enabled
# Should return: true
```

### 3. Check EC2 Instance Configuration

Use the AWS CLI to verify the instance was launched with enclave options enabled:

```bash
aws ec2 describe-instances \
  --instance-ids <instance-id> \
  --query 'Reservations[].Instances[].EnclaveOptions'
```

Expected output:

```json
[
  {
    "Enabled": true
  }
]
```

### 4. Deploy a Test Workload

Deploy a pod that requests the enclave extended resource:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: enclave-test
spec:
  containers:
  - name: test
    image: busybox
    command: ["sleep", "3600"]
    resources:
      requests:
        aws.amazon.com/nitro-enclaves: "1"
      limits:
        aws.amazon.com/nitro-enclaves: "1"
```

Verify the pod schedules successfully:

```bash
kubectl get pod enclave-test -o wide
# Should show the pod running on an enclave-enabled node
```

## Additional Resources

- [AWS Nitro Enclaves Documentation](https://docs.aws.amazon.com/enclaves/)
- [Nitro Enclaves SDK](https://github.com/aws/aws-nitro-enclaves-sdk-c)
- [EC2NodeClass Reference]({{< relref "../concepts/nodeclasses#specenclaveoptions" >}})
- [NodePool Reference]({{< relref "../concepts/nodepools" >}})
- [Karpenter Disruption Documentation]({{< relref "../concepts/disruption" >}})

## Follow-up

If you have questions or issues with Nitro Enclaves in Karpenter, feel free to:
- Open an issue on [GitHub](https://github.com/aws/karpenter-provider-aws/issues/new/choose)
- Ask in the [Karpenter Slack channel](https://kubernetes.slack.com/archives/C02SFFZSA2K)
- Check the [Troubleshooting Guide]({{< ref "../troubleshooting" >}})

