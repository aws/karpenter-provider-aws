# Karpenter v1beta1 APIs

This document formalizes the [v1beta1 laundry list](https://github.com/aws/karpenter/issues/1327) and describes the high-level migration strategy for users moving to v1beta1. It shows the full API specs, including Group/Kind names and label names. This document does not go into explicit details on each of the individual changes in v1beta1. For details on these individual changes, see [Karpenter v1beta1  Full Change List](./v1beta1-full-changelist.md).

## Bake Time

API changes create a user migration burden that should be weighed against the benefits of the breaking changes. Batching breaking changes into a single version bump **helps to minimize this burden**. The v1alpha5 API has seen broad adoption over the last year, and resulted in a large amount of feedback. We see this period to have been a critical maturation process for the Karpenter project, and has given us confidence that the changes in v1beta1 will be sufficient to promote after a shorter feedback period.

## Migration

Kubernetes custom resources have built-in support for API version compatibility. CRDs with multiple versions must define a “storage version”, which controls the data stored in etcd. Other versions are views onto this data and converted using [conversion webhooks](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion). However, there is a fundamental limitation that [all versions must be safely round-trippable through each other](https://book.kubebuilder.io/multiversion-tutorial/api-changes.html)[.](https://book.kubebuilder.io/multiversion-tutorial/api-changes.html) This means that it must be possible to define a function that converts a v1alpha5 Provisioner into a v1beta1 Provisioner and vise versa.

Unfortunately, multiple proposed changes in v1beta1 are not round-trippable. Below, we propose deprecations of legacy fields in favor more modern mechanisms that have seen adoption in v1alpha5. These changes remove sharp edges that regularly cause users surprises and production pain.

To workaround the limitation of round-trippability, we are proposing a rename of the Kinds (`NodePool`, `NodeClaim`, and `EC2NodeClass`) that the CRDs exist within. This allows both CRDs to exist alongside each other simultaneously and gives users a natural migration path to move through.

### Migration Path

Below describes a few migration paths at a high-level. These paths are not comprehensive, but offer good guidance through which users might migrate between the v1alpha5 APIs and the v1beta1 APIs.

#### Periodic Rolling with Drift

For each Provisioner in your cluster, perform the following actions:

1. Create a NodePool/NodeClass in your cluster that is the v1beta1 equivalent of the v1alpha5 Provisioner/AWSNodeTemplate
2. Add a taint to the old Provisioner such as `karpenter.sh/legacy=true:NoSchedule`
3. Karpenter drift will mark all machines/nodes owned by that Provisioner as drifted
4. Karpenter drift will launch replacements for the nodes in the new NodePool resource
   1. Currently, Karpenter only supports rolling of one node at a time, which means that it may take some time for Karpenter to completely roll all nodes under a single Provisioner

#### Forced Deletion

For each Provisioner in your cluster, perform the following actions:

1. Create a NodePool/NodeClass in your cluster that is the v1beta1 equivalent of the v1alpha5 Provisioner/AWSNodeTemplate
2. Delete the old Provisioner with `kubectl delete provisioner <provisioner-name> --cascade=foreground`
   1. Karpenter will delete each Node that is owned by the Provisioner, draining all nodes simultaneously and will launch nodes for the newly pending pods as soon as the Nodes enter a draining state

#### Manual Rolling

For each Provisioner in your cluster, perform the following actions:

1. Create a NodePool/NodeClass in your cluster that is the v1beta1 equivalent of the v1alpha5 Provisioner/AWSNodeTemplate
2. Add a taint to the old Provisioner such as `karpenter.sh/legacy=true:NoSchedule`
3. Delete each node one-at-time owned by the Provisioner by running `kubectl delete node <node-name>`

## APIs

To help clearly define where configuration should live within Karpenter’s API, we should define the logical boundary between each Kind in the project.

1. `NodePool`
    1. Neutral Node configuration-based fields that affect the **compatibility between Nodes and Pods during scheduling** (e.g. requirements, taints, labels)
    2. Neutral behavior-based fields for configuring Karpenter’s scheduling and deprovisioning decision-making
2. `EC2NodeClass`
    1. Cloudprovider-specific Node configuration-based fields that affect launch and bootstrap process for that Node including: configuring startup scripts, volume mappings, metadata settings, etc.
    2. Cloudprovider-specific behavior-based fields for configuring Karpenter’s scheduling and deprovisioning decision-making (e.g. interruption-based disruption, allocation strategy)
3. `NodeClaim`
    1. A Karpenter management object that fully manages the lifecycle of a single node including: configuring and launching the node, monitoring the node health (including disruption conditions), and handling the deprovisioning and termination of the node

With these boundaries defined, below shows each API, with all fields specified, with values filled in as examples.

### `karpenter.sh/NodePool`

```
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec:
  template:
    metadata:
      labels:
        billing-team: my-team
      annotations:
        example.com/owner: "my-team"
    spec:
      nodeClass:
        name: default
        kind: EC2NodeClass
        apiVersion: karpenter.k8s.aws/v1beta1
      taints:
        - key: example.com/special-taint
          effect: NoSchedule
      startupTaints:
        - key: example.com/another-taint
          effect: NoSchedule
      requirements:
        - key: "karpenter.k8s.aws/instance-category"
          operator: In
          values: ["c", "m", "r"]
      resources:
        requests:
          cpu: "1"
          memory: "100Mi"
      kubeletConfiguration:
        clusterDNS: ["10.0.1.100"]
        containerRuntime: containerd
        systemReserved:
          cpu: 100m
          memory: 100Mi
          ephemeral-storage: 1Gi
        kubeReserved:
          cpu: 200m
          memory: 100Mi
          ephemeral-storage: 3Gi
        evictionHard:
          memory.available: 5%
          nodefs.available: 10%
          nodefs.inodesFree: 10%
        evictionSoft:
          memory.available: 500Mi
          nodefs.available: 15%
          nodefs.inodesFree: 15%
        evictionSoftGracePeriod:
          memory.available: 1m
          nodefs.available: 1m30s
          nodefs.inodesFree: 2m
        evictionMaxPodGracePeriod: 60
        imageGCHighThresholdPercent: 85
        imageGCLowThresholdPercent: 80
        cpuCFSQuota: true
        podsPerCore: 2
        maxPods: 20
  disruption:
    consolidateAfter: 10m
    consolidationPolicy: WhenEmpty | WhenUnderutilized
    expireAfter: 30d
  weight: 10
  limits:
    cpu: "1000"
    memory: 1000Gi
status:
  resources:
     cpu: "2"
     memory: "100Mi"
     ephemeral-storage: "100Gi"
```

### `karpenter.k8s.aws/EC2NodeClass`

```
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: default
spec:
  amiFamily: AL2
  amiSelectorTerms:
    - tags: 
        key: value
    - id: abc-123
    - name: foo
      owner: amazon
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: cluster-name
    - id: subnet-1234
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: cluster-name
    - name: default-security-group
  role: karpenter-node-role
  userData: |
    echo "this is custom user data"
  tags:
    custom-tag: custom-value
  metadataOptions:
    httpEndpoint: enabled
    httpProtocolIPv6: disabled
    httpPutResponseHopLimit: 2
    httpTokens: required
  blockDeviceMappings:
    - deviceName: /dev/xvda
      ebs:
        volumeSize: 20Gi
        volumeType: gp3
        encrypted: true
  detailedMonitoring: true
status:
  subnets:
    - id: subnet-0a462d98193ff9fac
      zone: us-east-2b
    - id: subnet-0322dfafd76a609b6
      zone: us-east-2c
    - id: subnet-0727ef01daf4ac9fe
      zone: us-east-2b
    - id: subnet-00c99aeafe2a70304
      zone: us-east-2a
    - id: subnet-023b232fd5eb0028e
      zone: us-east-2c
    - id: subnet-03941e7ad6afeaa72
      zone: us-east-2a
  securityGroups:
    - id: sg-041513b454818610b
      name: ClusterSharedNodeSecurityGroup
    - id: sg-0286715698b894bca
      name: ControlPlaneSecurityGroup-1AQ073TSAAPW
  amis:
    - id: ami-05a05e85b17bb60d7
      name: amazon-eks-node-1.24-v20230703
      requirements:
        - key: karpenter.k8s.aws/instance-accelerator-count
          operator: DoesNotExist
        - key: kubernetes.io/arch
          operator: In
          values:
            - amd64
        - key: karpenter.k8s.aws/instance-gpu-count
          operator: DoesNotExist
    - id: ami-0d849ef1e65103147
      name: amazon-eks-gpu-node-1.24-v20230703
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - amd64
        - key: karpenter.k8s.aws/instance-accelerator-count
          operator: Exists
    - id: ami-0d849ef1e65103147
      name: amazon-eks-gpu-node-1.24-v20230703
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
           - amd64
        - key: karpenter.k8s.aws/instance-gpu-count
          operator: Exists
    - id: ami-0c3487f30d003deb3
      name: amazon-eks-arm64-node-1.24-v20230703
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - arm64
        - key: karpenter.k8s.aws/instance-gpu-count
          operator: DoesNotExist
        - key: karpenter.k8s.aws/instance-accelerator-count
          operator: DoesNotExist
```

### `karpenter.sh/NodeClaim`

```
apiVersion: karpenter.sh/v1beta1
kind: NodeClaim
metadata:
  name: default
  labels:
    billing-team: my-team
  annotations:
    example.com/owner: "my-team"
spec:
  nodeClass:
    name: default
    kind: EC2NodeClass
    apiVersion: karpenter.k8s.aws/v1beta1
  taints:
    - key: example.com/special-taint
      effect: NoSchedule
  startupTaints:
    - key: example.com/another-taint
      effect: NoSchedule
  requirements:
    - key: "karpenter.k8s.aws/instance-category"
      operator: In
      values: ["c", "m", "r"]
  resources:
    requests:
      cpu: "1"
      memory: "100Mi"
  kubeletConfiguration:
    clusterDNS: ["10.0.1.100"]
    containerRuntime: containerd
    systemReserved:
      cpu: 100m
      memory: 100Mi
      ephemeral-storage: 1Gi
    kubeReserved:
      cpu: 200m
      memory: 100Mi
      ephemeral-storage: 3Gi
    evictionHard:
      memory.available: 5%
      nodefs.available: 10%
      nodefs.inodesFree: 10%
    evictionSoft:
      memory.available: 500Mi
      nodefs.available: 15%
      nodefs.inodesFree: 15%
    evictionSoftGracePeriod:
      memory.available: 1m
      nodefs.available: 1m30s
      nodefs.inodesFree: 2m
    evictionMaxPodGracePeriod: 60
    imageGCHighThresholdPercent: 85
    imageGCLowThresholdPercent: 80
    cpuCFSQuota: true
    podsPerCore: 2
    maxPods: 20
status:
  allocatable:
    cpu: 1930m
    ephemeral-storage: 17Gi
    memory: 534108Ki
    pods: "4"
  capacity:
    cpu: "2"
    ephemeral-storage: 20Gi
    memory: 942684Ki
    pods: "4"
  conditions:
  - type: Drifted
    status: "True"
    severity: Warning
  - status: "True"
    type: Initialized
  - status: "True"
    type: Lanched
  - status: "True"
    type: Registered
  - status: "True"
    type: Ready
  nodeName: ip-192-168-62-137.us-west-2.compute.internal
  providerID: aws:///us-west-2a/i-08168021ae532fca3
```

### Labels/Annotations

#### `karpenter.sh`

1. `karpenter.sh/nodepool`
2. `karpenter.sh/initialized`
3. `karpenter.sh/registered`
4. `karpenter.sh/capacity-type`
5. `karpenter.sh/do-not-disrupt`

#### `karpenter.k8s.aws`

1. `karpenter.k8s.aws/instance-hypervisor`
2. `karpenter.k8s.aws/instance-encryption-in-transit-supported`
3. `karpenter.k8s.aws/instance-category`
4. `karpenter.k8s.aws/instance-family`
5. `karpenter.k8s.aws/instance-generation`
6. `karpenter.k8s.aws/instance-local-nvme`
7. `karpenter.k8s.aws/instance-size`
8. `karpenter.k8s.aws/instance-cpu`
9. `karpenter.k8s.aws/instance-cpu-manufacturer`
10. `karpenter.k8s.aws/instance-memory`
11. `karpenter.k8s.aws/instance-ebs-bandwidth`
11. `karpenter.k8s.aws/instance-network-bandwidth`
12. `karpenter.k8s.aws/instance-gpu-name`
13. `karpenter.k8s.aws/instance-gpu-manufacturer`
14. `karpenter.k8s.aws/instance-gpu-count`
15. `karpenter.k8s.aws/instance-gpu-memory`
16. `karpenter.k8s.aws/instance-accelerator-name`
17. `karpenter.k8s.aws/instance-accelerator-manufacturer`
18. `karpenter.k8s.aws/instance-accelerator-count`
