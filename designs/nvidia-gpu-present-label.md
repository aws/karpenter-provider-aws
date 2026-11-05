# `nvidia.com/gpu.present` as a Well-Known Label

## Background

Karpenter only launches a node for a pod whose selector it can predict. It predicts its *well-known labels*
from each instance type's requirements; any other label a pod selects on must be set by hand in a NodePool's
`spec.template.metadata.labels`. Every well-known label the AWS provider registers today is in a Kubernetes,
AWS or Karpenter domain ([labels.go](https://github.com/aws/karpenter-provider-aws/blob/main/pkg/apis/v1/labels.go)).

`nvidia.com/gpu.present=true` marks a node with NVIDIA GPU hardware. NVIDIA components select on it: the
GPU Operator's DRA driver DaemonSet requires it
([0500_daemonset.yaml](https://github.com/NVIDIA/gpu-operator/blob/main/manifests/state-dra-driver/0500_daemonset.yaml)),
and it is one of the three alternatives in the device plugin chart's default node affinity
([values.yaml](https://github.com/NVIDIA/k8s-device-plugin/blob/main/deployments/helm/nvidia-device-plugin/values.yaml)).
A pod that selects it today never causes Karpenter to launch a node, even though Karpenter already knows which
instance types have an NVIDIA GPU (`karpenter.k8s.aws/instance-gpu-manufacturer=nvidia`).

This RFC covers registering `nvidia.com/gpu.present` as a well-known label
([#9817](https://github.com/aws/karpenter-provider-aws/pull/9817)), and the longer-term direction for vendor
labels.

## Proposal

Register `nvidia.com/gpu.present` as a well-known label. Instance types with an NVIDIA GPU (EC2
`GpuInfo` manufacturer `NVIDIA`, the same source as `instance-gpu-manufacturer`) get `In ["true"]`; every other
instance type gets `DoesNotExist`. Registering `DoesNotExist` matters: `Requirements.Get` on a missing key
returns `Exists`, so omitting it would let a pod requiring the label match an instance type whose node never
carries it.

The single-valued requirement becomes a NodeClaim label, and registration copies NodeClaim labels to the Node,
so the Node carries the label on every Kubernetes version. No user data change is needed.

## Semantics and ordering

`nvidia.com/gpu.present` is a hardware-presence label, not a readiness signal. Every existing source sets it
before the GPU software stack is ready:

- The NVIDIA GPU Operator keeps it in sync with Node Feature Discovery's GPU PCI labels, and only applies its
  `nvidia.com/gpu.deploy.*` labels (which place the driver and other operands) to nodes that carry it
  ([nodelabeling_controller.go](https://github.com/NVIDIA/gpu-operator/blob/main/controllers/nodelabeling_controller.go)).
- AL2023 nodeadm sets it as a kubelet registration label when an NVIDIA PCI device is attached, on kubelet
  v1.35.0 and later
  ([config.go](https://github.com/awslabs/amazon-eks-ami/blob/main/nodeadm/internal/kubelet/config.go)).
- Bottlerocket NVIDIA variants hard-code it into the kubelet's `NODE_LABELS` on Kubernetes 1.35 and later
  ([kubelet-env-nvidia](https://github.com/bottlerocket-os/bottlerocket-core-kit/blob/develop/packages/kubernetes-1.35/kubelet-env-nvidia)).

So a DaemonSet that selects on the label already lands before the GPU stack is ready on these nodes, and has to
tolerate that today. Karpenter setting it at launch doesn't change that. Workloads that need a working GPU
request the `nvidia.com/gpu` extended resource, which is only advertised once the device plugin is running.

Karpenter owns only the instance-type-to-value mapping; NVIDIA owns what the label means.

## Compatibility

1. **NodePools that already set the label.** NodePool template labels become `In` requirements. A NodePool that
   sets `nvidia.com/gpu.present: "true"` becomes limited to NVIDIA GPU instance types; one that sets `"false"`,
   or that offers only non-NVIDIA instance types while setting `"true"`, can no longer launch nodes.
2. **DaemonSets on new nodes.** NVIDIA DaemonSets that select on the label start scheduling onto new NVIDIA GPU
   nodes that weren't labeled by hand. On AL2023 and Bottlerocket on 1.35+, those nodes already carry the label
   from the image, so this mostly affects older Kubernetes versions and custom AMIs.
3. **Existing nodes.** Existing NodeClaims and Nodes aren't relabeled, and aren't drifted: requirements drift
   compares NodePool requirements against NodeClaim labels, not against instance-type requirements.

## Why not `karpenter.k8s.aws/instance-gpu-manufacturer`

Workloads users write can already select on `instance-gpu-manufacturer=nvidia`. The gap is third-party
components whose defaults select on `nvidia.com/gpu.present`. The workarounds, overriding each component's
affinity or setting the label on each NodePool, are per-component and per-NodePool, and are easy to miss in
clusters that don't run Node Feature Discovery.

## Long-term: out-of-tree vendor integration

Karpenter already couples to the NVIDIA stack through the `nvidia.com/gpu` resource, the
`karpenter.k8s.aws/nvidia-consumable-capacity` annotation, and the `DRANVIDIAGPU` feature gate; this label
extends that coupling rather than starting it. Longer term, vendor-specific labels and resources should move to
an out-of-tree model, where a vendor integration supplies its own instance-type labels and resources instead of
each one being compiled into the provider. That model is out of scope here; until it exists, a vendor label
should only be added in-tree if:

- its value is derivable from EC2 instance-type data,
- the vendor already defines it as a hardware-presence label set before its software is ready,
- it agrees with what the supported AMI families set, and
- a widely used component selects on it by default.

## Enablement

The label is registered by default, with no feature gate. This matches how other well-known labels have been
added, including `karpenter.k8s.aws/instance-tenancy` (#8218), `karpenter.k8s.aws/instance-capability-flex`
(#8490), `karpenter.k8s.aws/capacity-reservation-interruptible` (#9019) and
`karpenter.k8s.aws/placement-group-id` (#9030). Compatibility change 1 isn't specific to this label: registering
any label as well-known constrains NodePools that already set it by hand. The only NodePools that stop launching
are ones that set `"false"`, or set `"true"` without offering NVIDIA instance types; both already contradict what
the GPU Operator, AL2023 and Bottlerocket set. The upgrade guide calls out NodePools that set the label by hand.
