# Karpenter v1beta1 APIs

This document formalizes the [v1beta1 laundry list](https://github.com/aws/karpenter/issues/1327) and describes the high-level migration strategy for users moving to v1beta1. It shows the full API specs, including Group/Kind names and label names. This document does not go into explicit details on each of the individual changes in v1beta1. For details on these individual changes, see [Karpenter v1beta1  Full Change List](./v1beta1-full-changelist.md).

## Bake Time

API changes create a user migration burden that should be weighed against the benefits of the breaking changes. Batching breaking changes into a single version bump **helps to minimize this burden**. The v1alpha5 API has seen broad adoption over the last year, and resulted in a large amount of feedback. We see this period to have been a critical maturation process for the Karpenter project, and has given us confidence that the changes in v1beta1 will be sufficient to promote after a shorter feedback period.

## Migration

**Tenet:** Customers will be able to migrate from v1alpha5 to v1beta1 in a single cluster using any Karpenter version from the time that we release v1beta1 up until we release v1.

Kubernetes custom resources have built in support for API version compatibility. CRDs with multiple versions must define a “storage version”, which controls the data stored in etcd. Other versions are views onto this data and converted using [conversion webhooks](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#webhook-conversion). However, there is a fundamental limitation that [all versions must be safely round-trippable through each other](https://book.kubebuilder.io/multiversion-tutorial/api-changes.html)[.](https://book.kubebuilder.io/multiversion-tutorial/api-changes.html) This means that it must be possible to define a function that converts a v1alpha5 Provisioner into a v1beta1 Provisioner and vise versa.

Unfortunately, multiple proposed changes in v1beta1 are not round-trippable. Below, we propose deprecations of legacy fields in favor more modern mechanisms that have seen adoption in v1alpha5. These changes remove sharp edges that regularly cause users surprises and production pain.

To workaround the limitation of round-trippability, we are proposing a rename of both the API group and Kind (`NodePool`) that the CRDs exist within. This allows both CRDs to exist alongside each other simultaneously and gives users a natural migration path to move through.

### Migration Path

#### Aggressive Scale-Down

1. Spin up a new `v1beta1/NodePool` and `v1beta1/NodeTemplate` resource matching their `v1alpha5` counterparts
2. Delete the `v1alpha5/Provisioner` and `v1alpha5/AWSNodeTemplate` with [cascading foreground deletion](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#foreground-deletion) to delete the nodes and have Karpenter roll the nodes to `v1beta1`

#### Slow Scale-Down

1. Spin up a new `v1beta1/NodePool` and `v1beta1/NodeTemplate` resource matching their `v1alpha5` counterparts
2. Set the `spec.limits` on your `v1alpha5/Provisioner` resource to `cpu=0` to stop provisioning
3. Manually delete Nodes one-by-one to have Karpenter roll the nodes over to `v1beta1`

## APIs

To help clearly define where configuration should live within Karpenter’s API, we should define the logical boundary between each Kind in the project.

1. `NodePool`
    1. Neutral Node configuration-based fields that affect the **compatibility between Nodes and Pods during scheduling** (e.g. requirements, taints, labels)
    2. Neutral behavior-based fields for configuring Karpenter’s scheduling and deprovisioning decision-making
2. `NodeTemplate`
    1. Cloudprovider-specific Node configuration-based fields that affect launch and bootstrap process for that Node including: configuring startup scripts (including kubelet configuration), volume mappings, metadata settings, etc.
    2. Cloudprovider-specific behavior-based fields for configuring Karpenter’s scheduling and deprovisioning decision-making (e.g. interruption-based disruption, allocation strategy)
3. `Machine`
    1. A Karpenter management object that fully manages the lifecycle of a single node including: configuring and launching the node, monitoring the node health (including disruption conditions), and handling the deprovisioning and termination of the node

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
      nodeTemplateRef:
        name: default
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
      resources: # Most users wouldn't set this field but it's in the MachineSpec
        requests:
          cpu: "1"
          memory: "100Mi"
  deprovisioning:
    ttlAfterUninitialized: 10m
    ttlAfterUnderutilized: 30s
    ttlUntilExpired: 30d
    consolidation:
      enabled: true/false
  weight: 10
  limits:
    cpu: "1000"
    memory: 1000Gi
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
  resources:
     cpu: "2"
     memory: "100Mi"
     ephemeral-storage: "100Gi"
```

### `compute.k8s.aws/NodeTemplate`

```
apiVersion: compute.k8s.aws/v1beta1
kind: NodeTemplate
metadata:
  name: default
spec:
  amiFamily: AL2
  amiSelector:
    - tags: 
        key: value
    - id: abc-123
    - name: foo
      owner: amazon
    - ssm: "/my/custom/parameter"
  subnetSelector:
    - tags:
        compute.k8s.aws/discovery: cluster-name
    - id: subnet-1234
  securityGroupSelector:
    - tags:
        compute.k8s.aws/discovery: cluster-name
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

### `karpenter.sh/Machine`

```
apiVersion: karpenter.sh/v1beta1
kind: Machine
metadata:
  name: default
  labels:
    billing-team: my-team
  annotations:
    example.com/owner: "my-team"
spec:
  nodeTemplateRef:
    name: default
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
  - type: MachineDrifted
    status: "True"
    severity: Warning
  - status: "True"
    type: MachineInitialized
  - status: "True"
    type: MachineLaunched
  - status: "True"
    type: MachineRegistered
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

#### `compute.k8s.aws`

1. `compute.k8s.aws/instance-hypervisor`
2. `compute.k8s.aws/instance-encryption-in-transit-supported`
3. `compute.k8s.aws/instance-category`
4. `compute.k8s.aws/instance-family`
5. `compute.k8s.aws/instance-generation`
6. `compute.k8s.aws/instance-local-nvme`
7. `compute.k8s.aws/instance-size`
8. `compute.k8s.aws/instance-cpu`
9. `compute.k8s.aws/instance-memory`
10. `compute.k8s.aws/instance-network-bandwidth`
11. `compute.k8s.aws/instance-pods`
12. `compute.k8s.aws/instance-gpu-name`
13. `compute.k8s.aws/instance-gpu-manufacturer`
14. `compute.k8s.aws/instance-gpu-count`
15. `compute.k8s.aws/instance-gpu-memory`
16. `compute.k8s.aws/instance-accelerator-name`
17. `compute.k8s.aws/instance-accelerator-manufacturer`
18. `compute.k8s.aws/instance-accelerator-count`
