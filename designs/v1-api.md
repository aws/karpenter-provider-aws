# Karpenter v1 API

_This RFC is an extension of the [v1 API RFC](https://github.com/kubernetes-sigs/karpenter/blob/main/designs/v1-api.md) that is merged in the [`kubernetes-sigs/karpenter` repo](https://github.com/kubernetes-sigs/karpenter)._

## Overview

Karpenter released the beta version of its APIs and features in October 2023. The intention behind this beta was that we would be able to determine the final set of changes and feature adds that we wanted to add to Karpenter before we considered Karpenter feature-complete. The list below details the features that Karpenter has on its roadmap before Karpenter becomes feature complete and stable at v1.

### Categorization

This list represents the minimal set of changes that are needed to ensure proper operational excellence, feature completeness, and stability by v1. For a change to make it on this list, it must meet one of the following criteria:

1. Breaking: The feature requires changes or removals from the API that would be considered breaking after a bump to v1
2. Stability: The feature ensures proper operational excellence for behavior that is leaky or has race conditions in the beta state
3. Planned Deprecations: The feature cleans-up deprecations that were previously planned the project

## EC2NodeClass API

```
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: default
spec:
  kubelet:
    podsPerCore: 2
    maxPods: 20
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
   clusterDNS: ["10.0.1.100"]
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
    - id: subnet-09fa4a0a8f233a921
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
    - name: my-security-group
    - id: sg-063d7acfb4b06c82c
  amiFamily: AL2023
  amiSelectorTerms:
    - alias: al2023@v20240625
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}"
    - name: my-ami
    - id: ami-123
  role: "KarpenterNodeRole-${CLUSTER_NAME}"
  instanceProfile: "KarpenterNodeInstanceProfile-${CLUSTER_NAME}"
  userData: |
    echo "Hello world"
  tags:
    team: team-a
    app: team-a-app
  instanceStorePolicy: RAID0
  metadataOptions:
    httpEndpoint: enabled
    httpProtocolIPv6: disabled
    httpPutResponseHopLimit: 1 # This is changed to disable IMDS access from containers not on the host network
    httpTokens: required
  blockDeviceMappings:
    - deviceName: /dev/xvda
      ebs:
        volumeSize: 100Gi
        volumeType: gp3
        iops: 10000
        encrypted: true
        kmsKeyID: "1234abcd-12ab-34cd-56ef-1234567890ab"
        deleteOnTermination: true
        throughput: 125
        snapshotID: snap-0123456789
  detailedMonitoring: **true**
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
    - id: ami-01234567890123456
      name: custom-ami-amd64
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - amd64
    - id: ami-01234567890123456
      name: custom-ami-arm64
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values:
            - arm64
  instanceProfile: "${CLUSTER_NAME}-0123456778901234567789"
  conditions:
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: InstanceProfileReady
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: SubnetsReady
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: SecurityGroupsReady
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: AMIsReady
    - lastTransitionTime: "2024-02-02T19:54:34Z"
      status: "True"
      type: Ready
```

### Printer Columns

**Category:** Stability, Breaking

#### Current

```
➜  karpenter git:(main) ✗ k get ec2nodeclasses -o wide
NAME      AGE
default   2d8h
```

#### Proposed

```
➜  karpenter git:(main) ✗ k get ec2nodeclasses -o wide
NAME     READY  AGE    ROLE
default  True   2d8h   KarpenterNodeRole-test-cluster
```

**Standard Columns**

1. Name
3. Ready - EC2NodeClasses now have status conditions that inform the user whether the EC2NodeClass has resolved all of its data and is “ready” to be used by a NodePool. This readiness should be easily viewable by users.
4. Age

**Wide Columns (-o wide)**

1. Role - As a best practice, we are recommending that users use a Node role and let Karpenter create a managed instance profile on behalf of the customer. We should easily expose this role.

#### Status Conditions

**Category:** Stability

Defining the complete set of status condition types that we will include on v1 launch is **out of scope** of this document and will be defined with more granularly in Karpenter’s Observability design. Minimally for v1, we will add a `Ready` condition so that we can determine whether a EC2NodeClass can be used by a NodePool during scheduling. More robustly, we will define status conditions that ensure that each required “concept” that’s needed for an instance launch is resolved e.g. InstanceProfile resolved, Subnet resolved, Security Groups resolved, etc.

#### Require AMISelectorTerms

**Category:** Stability, Breaking

When specifying AMIFamily with no AMISelectorTerms, users are currently configured to automatically update AMIs when a new version of the EKS-optimized image in that family is released. Existing nodes on older versions of the AMI will drift to the newer version to meet the desired state of the EC2NodeClass.

This works well in pre-prod environments where it’s nice to get auto-upgraded to the latest version for testing but is extremely risky in production environments. [Karpenter now recommends to users to pin AMIs in their production environments](https://karpenter.sh/docs/tasks/managing-amis/#option-1-manage-how-amis-are-tested-and-rolled-out:~:text=The%20safest%20way%2C%20and%20the%20one%20we%20recommend%2C%20for%20ensuring%20that%20a%20new%20AMI%20doesn%E2%80%99t%20break%20your%20workloads%20is%20to%20test%20it%20before%20putting%20it%20into%20production); however, it’s still possible to be caught by surprise today that Karpenter has this behavior when you deploy a EC2NodeClass and NodePool with an AMIFamily. Most notably, this is different from eksctl and MNG, where they will get the latest AMI when you first deploy the node group, but will pin it at the point that you add it.

We no longer want to deal with potential confusion around whether nodes will get rolled or not when using an AMIFamily with no `amiSelectorTerms`. Instead, `amiSelectorTerms` will now be required and a new term type, `alias`, will be introduced which allows users to select an EKS optimized AMI. Each alias consists of an AMI family and a version. Users can set the version to `latest` to continue to get automatic upgrades, or pin to a specific version.

#### Disable IMDS Access from Containers by Default

**Category:** Stability, Breaking

The HTTPPutResponseHopLimit is [part of the instance metadata settings that are configured on the node on startup](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-options.html). This setting dictates how many hops a PUT request can take before it will be rejected by IMDS. For Kubernetes pods that live in another network namespace, this means that any pod that isn’t using `hostNetwork: true` [would need to have a HopLimit of 2 set in order to access IMDS](https://aws.amazon.com/about-aws/whats-new/2020/08/amazon-eks-supports-ec2-instance-metadata-service-v2/#:~:text=However%2C%20this%20limit%20is%20incompatible%20with%20containerized%20applications%20on%20Kubernetes%20that%20run%20in%20a%20separate%20network%20namespace%20from%20the%20instance). Opening up the node for pods to reach out to IMDS is an inherent security risk. If you are able to grab a token for IMDS, you can craft a request that gives the pod the same level of access as the instance profile which orchestrates the kubelet calls on the cluster.

We should constrain our pods to not have access to IMDS by default to not open up users to this security risk. This new default wouldn’t affect users who have already deployed EC2NodeClasses on their cluster. It would only affect new EC2NodeClasses.

## Labels/Annotations/Tags

#### karpenter.sh/managed-by (EC2 Instance Tag)

**Category:** Planned Deprecations, Breaking

Karpenter introduced the `karpenter.sh/managed-by` tag in v0.28.0 when migrating Karpenter over to NodeClaims (called Machines at the time). This migration was marked as “completed” when it tagged the instance in EC2 with the `karpenter.sh/managed-by` tag and stored the cluster name as the value. Since we have completed the NodeClaim migration, we no longer have a need for this tag; so, we can drop it.

This tag was only useful for scoping pod identity policies with ABAC, since it stored the cluster name in the value rather than `kubernetes.io/cluster/<cluser-name>` which stores the cluster name in the tag key. Session tags don’t work with tag keys, so we need some tag that we can recommend users to use to create pod identity policies with ABAC using OSS Karpenter.

Starting in v1, Karpenter would use `eks:eks-cluster-name: <cluster-name>` for tagging and scoping instances, volumes, primary ENIs, etc. and would use `eks:eks-cluster-arn: <cluster-arn>` for tagging and scoping instance profiles that it creates.