---
title: "Threat Model"
linkTitle: "Threat Model"
weight: 999
---

Karpenter observes Kubernetes pods and launches nodes in response to those pods’ scheduling constraints. Karpenter does not perform the actual scheduling and instead waits for [kube-scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/) to schedule the pods.

When running in AWS, Karpenter is typically installed onto EC2 instances that run in EKS Clusters. Karpenter relies on public facing AWS APIs and standard IAM Permissions. Karpenter uses AWS-SDK-Go v1, and AWS advises that credentials are provided using [IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html).


## Architecture & Actors

1. **Cluster Operator**: An identity that installs and configures Karpenter in a Kubernetes cluster, and configures Karpenter's cloud identity and permissions.
2. **Cluster Developer**: An identity that can create pods, typically through Deployments, DaemonSets, or other pod-controller types.
3. **Karpenter Controller:** The Karpenter application pod that operates inside a cluster.

![threat-model](/threat-model.png)

## Capabilities

### Cluster Operator

The Cluster Operator has full control to install and configure Karpenter including all [`NodePools`]({{<ref "../concepts/nodepools" >}}) and [`EC2NodeClasses`]({{<ref "../concepts/nodeclasses" >}}). The Cluster Operator has privileges to manage the cloud identities and permissions for Nodes, and the cloud identity and permissions for Karpenter.

### Cluster Developer

A Cluster Developer has the ability to create pods via `Deployments`, `ReplicaSets`, `StatefulSets`, `Jobs`, etc. This assumes that the Cluster Developer cannot modify the Karpenter pod or launch pods using Karpenter’s service account and gain access to Karpenter’s IAM role.

### Karpenter Controller

Karpenter has permissions to create and manage cloud instances. Karpenter has Kubernetes API permissions to create, update, and remove nodes, as well as evict pods. For a full list of the permissions, see the RBAC rules in the helm chart template. Karpenter also has AWS IAM permissions to create instances with IAM roles.

* [aggregate-clusterrole.yaml](https://github.com/aws/karpenter/blob/v1.1.0/charts/karpenter/templates/aggregate-clusterrole.yaml)
* [clusterrole-core.yaml](https://github.com/aws/karpenter/blob/v1.1.0/charts/karpenter/templates/clusterrole-core.yaml)
* [clusterrole.yaml](https://github.com/aws/karpenter/blob/v1.1.0/charts/karpenter/templates/clusterrole.yaml)
* [rolebinding.yaml](https://github.com/aws/karpenter/blob/v1.1.0/charts/karpenter/templates/rolebinding.yaml)
* [role.yaml](https://github.com/aws/karpenter/blob/v1.1.0/charts/karpenter/templates/role.yaml)

## Assumptions

| Category	     | Assumption	                                                                                                                                                                                                            | Comment	                                                                                                                                                                                                                          |
|---------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Generic	      | The Karpenter pod is operated on a node in the cluster, and uses a Service Account for authentication to the Kubernetes API	                                                                                           | Cluster Operators may want to isolate the node running the Karpenter pod to a system-pool of nodes to mitigate the possibility of container breakout with Karpenter’s permissions. 	                                              |
| Generic	      | Cluster Developer does not have any Kubernetes permissions to manage Karpenter running in the cluster (The deployment, pods, clusterrole, etc)	                                                                        | 	                                                                                                                                                                                                                                 |
| Generic	      | Restrictions on the fields of pods a Cluster Developer can create are out of scope. 	                                                                                                                                  | Cluster Operators can use policy frameworks to enforce restrictions on Pod capabilities	                                                                                                                                          |
| Generic	      | No sensitive data is included in non-Secret resources in the Kubernetes API. The Karpenter controller has the ability to list all pods, nodes, deployments, and many other pod-controller and storage resource types.	 | Karpenter does not have permission to list/watch cluster-wide ConfigMaps or Secrets	                                                                                                                                              |
| Generic	      | Karpenter has permissions to create, modify, and delete nodes from the cluster, and evict any pod. 	                                                                                                                   | Cluster Operators running applications with varying security profiles in the same cluster may want to configure dedicated nodes and scheduling rules for Karpenter to mitigate potential container escapes from other containers	 |
| AWS-Specific	 | The Karpenter IAM policy is encoded in the GitHub repo. Any additional permissions possibly granted to that role by the administrator are out of scope	                                                                | 	                                                                                                                                                                                                                                 |
| AWS-Specific	 | The Karpenter pod uses IRSA for AWS credentials 	                                                                                                                                                                      | Setup of IRSA is out of scope for this document 	                                                                                                                                                                                 |

## Generic Threats and Mitigations

### Threat: Cluster Developer can influence creation of an arbitrary number of nodes

**Background**: Karpenter creates new instances based on the count of pending pods.

**Threat**: A Cluster Developer attempts to have Karpenter create more instances than intended by creating a large number of pods or by using anti-affinity to schedule one pod per node.

**Mitigation**: In addition to [Kubernetes resource limits](https://kubernetes.io/docs/concepts/policy/resource-quotas/#object-count-quota), Cluster Operators can [configure limits on a NodePool]({{< ref "../concepts/nodepools#spec-limits" >}}) to limit the total amount of memory, CPU, or other resources provisioned across all nodes.

## Threats

### Threat: Using EC2 CreateTag/DeleteTag Permissions to Orchestrate Instance Creation/Deletion

**Background**: As of `0.28.0`, Karpenter creates a mapping between CloudProvider instances and CustomResources in the cluster for capacity tracking. To ensure this mapping is consistent, Karpenter utilizes the following tag keys:

* `karpenter.sh/managed-by`
* `karpenter.sh/nodepool`
* `kubernetes.io/cluster/${CLUSTER_NAME}`
* `karpenter.sh/provisioner-name` (prior to `0.32.0`)

Any user that has the ability to Create/Delete tags on CloudProvider instances will have the ability to orchestrate Karpenter to Create/Delete CloudProvider instances as a side effect.

In addition, as of `0.29.0`, Karpenter will Drift on Security Groups and Subnets. If a user has the Create/Delete tags permission for either of resources, they can orchestrate Karpenter to Create/Delete CloudProvider instances as a side effect.

**Threat:** A Cluster Operator attempts to create or delete a tag on a resource discovered by Karpenter. If it has the ability to create a tag it can effectively create or delete CloudProvider instances associated with the tagged resources.

**Mitigation** Cluster Operators should [enforce tag-based IAM policies](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_tags.html) on these tags against any EC2 instance resource (`i-*`) for any users that might have [CreateTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateTags.html)/[DeleteTags](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DeleteTags.html) permissions but should not have [RunInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RunInstances.html)/[TerminateInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TerminateInstances.html) permissions.

### Threat: Launching EC2 instances with IAM roles not intended for Karpenter nodes

**Background**: Many IAM roles in an AWS account may trust the EC2 service principal. IAM administrators must grant the `iam:PassRole` permission to IAM principals to allow those principals in the account to launch instances with specific roles.

**Threat:** A Cluster Operator attempts to create an `EC2NodeClass` with an IAM role not intended for Karpenter

**Mitigation**: Cluster Operators must enumerate the roles in the resource section of the IAM policy granted to the Karpenter role for the `iam:PassRole` action. Karpenter will fail to generate an instance profile if role that is specified in the `spec.role` section of the `EC2NodeClass` is not enumerated in the `iam:PassRole` permission.

### Threat: Karpenter can orchestrate the creation/deletion of IAM Instance Profiles it doesn't own

**Background**: Karpenter has permission to create/update/delete instance profiles as part of its controller permissions to ensure that it can auto-generate instance profiles when EC2NodeClasses are created.

**Threat**: An actor who obtains control of the Karpenter pod’s IAM role may delete instance profiles not owned by Karpenter, causing workload disruption to other instances using the profile in the account.

**Mitigation**: Karpenter's controller permissions enforce that it creates instance profiles with tags which provide ownership information. These tags include:

* `karpenter.sh/managed-by`
* `kubernetes.io/cluster/${CLUSTER_NAME}`
* `karpenter.k8s.aws/ec2nodeclass`
* `topology.kubernetes.io/region`

These tags ensure that instance profiles created by Karpenter in the account are unique to that cluster. Karpenter's controller permissions _only_ allow it to act on instance profiles that contain these tags which match the cluster information.

### Threat: Karpenter can be used to create or terminate EC2 instances outside the cluster

**Background**: EC2 instances can exist in an AWS account outside of the Kubernetes cluster.

**Threat**: An actor who obtains control of the Karpenter pod’s IAM role may be able to create or terminate EC2 instances not part of the Kubernetes cluster managed by Karpenter.

**Mitigation**: Karpenter creates instances with tags, several of which are enforced in the IAM policy granted to the Karpenter IAM role that restrict the instances Karpenter can terminate. One tag requires that the instance was provisioned by a Karpenter controller (`karpenter.sh/nodepool`), another tag can include a cluster name to mitigate any termination between two clusters with Karpenter in the same account (`kubernetes.io/cluster/${CLUSTER_NAME}`. Cluster Operators also can restrict the region to prevent two clusters in the same account with the same name in different regions.

Additionally, Karpenter does not allow tags to be modified on instances unowned by Karpenter after creation, except for the `Name` and `karpenter.sh/nodeclaim` tags. Though these tags can be changed after instance creation, `aws:ResourceTag` conditions enforce that the Karpenter controller is only able to change these tags on instances that it already owns, enforced through the `karpenter.sh/nodepool` and `kubernetes.io/cluster/${CLUSTER_NAME}` tags.

### Threat: Karpenter launches an EC2 instance using an unintended AMI

**Background**: Cluster Developers can create Node Templates that refer to an AMI by metadata, such as a name rather than an AMI resource ID.

**Threat:** A threat actor creates a public AMI with the same name as a customer’s AMI in an attempt to get Karpenter to select the threat actor’s AMI instead of the intended AMI.

**Mitigation**: When selecting AMIs by name or tags, Karpenter defaults to adding an ownership filter of `self,amazon` so AMI images external to the account are not used.
