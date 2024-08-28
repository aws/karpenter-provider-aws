---
title: "Threat Model"
linkTitle: "Threat Model"
weight: 999
---

Karpenter is an open source dynamic provisioner and autoscaler for Kubernetes. It automatically launches just-in-time compute resources to handle your cluster's applications. It observes Kubernetes pods and launches nodes in response to those pods’ scheduling constraints. It  does not perform the actual scheduling and instead waits for [kube-scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/) to schedule the pods. 

Customers self deploy and manage the lifecycle of Karpenter in their clusters. When running in AWS, Karpenter is installed on either an [Managed Node Groups](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html) or a [Fargate Profile](https://docs.aws.amazon.com/eks/latest/userguide/fargate-profile.html) that runs in EKS Clusters. Karpenter relies on public facing AWS APIs and IAM Permissions. It uses AWS-SDK-Go v1, and AWS-vended credentials using [IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html) or [EKS Pod Identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html).

## Architecture & Actors

1. **Cluster Operator**: An identity that installs and configures Karpenter in a Kubernetes cluster, and configures Karpenter's cloud identity and permissions.
2. **Cluster Developer**: An identity that can create pods, typically through Deployments, DaemonSets, or other pod-controller types.
3. **Karpenter Controller:** The Karpenter application pod that operates inside a cluster.

![threat-model](/threat-model.jpg)

## Capabilities

### Cluster Operator

The Cluster Operator has full control to install and configure Karpenter including all [`NodePools`]({{<ref "../concepts/nodepools" >}}) and [`EC2NodeClasses`]({{<ref "../concepts/nodeclasses" >}}). The Cluster Operator has privileges to manage the cloud identities and permissions for Nodes, and the cloud identity and permissions for Karpenter.

### Cluster Developer

A Cluster Developer has the ability to create pods via `Deployments`, `ReplicaSets`, `StatefulSets`, `Jobs`, etc. This assumes that the Cluster Developer cannot modify the Karpenter pod or launch pods using Karpenter’s service account and gain access to Karpenter’s IAM role.

### Karpenter Controller

#### IAM Permission

**KarpenterControllerPolicy**

A KarpenterControllerPolicy object sets the name of the policy, then defines a set of resources and actions allowed for those resources. Below are the minimum permissions needed to operate Karpenter:

|          Write Operations         |          Read Operations          |
|:---------------------------------:|:---------------------------------:|
| ec2:CreateFleet                   | ec2:DescribeAvailabilityZones     |
| ec2:CreateLaunchTemplate          | ec2:DescribeImages                |
| ec2:CreateTags                    | ec2:DescribeInstances             |
| ec2:DeleteLaunchTemplate          | ec2:DescribeInstanceTypeOfferings |
| ec2:RunInstances                  | ec2:DescribeInstanceTypes         |
| ec2:TerminateInstances            | ec2:DescribeLaunchTemplates       |
| iam:PassRole                      | ec2:DescribeSecurityGroups        |
| iam:CreateInstanceProfile         | ec2:DescribeSpotPriceHistory      |
| iam:TagInstanceProfile            | ec2:DescribeSubnets               |
| iam:AddRoleToInstanceProfile      | pricing:GetProducts               |
| iam:RemoveRoleFromInstanceProfile | ssm:GetParameter                  |
| iam:DeleteInstanceProfile         | iam:GetInstanceProfile            |
| sqs:DeleteMessage (Optional)      | eks:DescribeCluster               |
|                                   | sqs:GetQueueAttributes(Optional)  |
|                                   | sqs:GetQueueUrl(Optional)         |
|                                   | sqs:ReceiveMessage(Optional)      |

For more information checkout [KarpenterControllerPolicy](https://karpenter.sh/docs/reference/cloudformation/#karpentercontrollerpolicy)

**KarpenterNodeRole**

The KarpenterNodeRole is created using several AWS managed policies, which are designed to provide permissions for specific uses needed by the nodes to work with EC2 and other AWS resources.

| Policy                             | Description                                                                                  |
|------------------------------------|----------------------------------------------------------------------------------------------|
| AmazonEKS_CNI_Policy               | Provides the permissions that the Amazon VPC CNI Plugin needs to configure EKS worker nodes. |
| AmazonEKSWorkerNodePolicy          | Lets Amazon EKS worker nodes connect to EKS Clusters.                                        |
| AmazonEC2ContainerRegistryReadOnly | Allows read-only access to repositories in the Amazon EC2 Container Registry.                |
| AmazonSSMManagedInstanceCore       | Adds AWS Systems Manager service core functions for Amazon EC2.                              |


For more information checkout [KarpenterNodeRole]( https://karpenter.sh/docs/reference/cloudformation/#karpenternoderoley)

#### RBAC Policies

Karpenter has Kubernetes API permissions to create, update, and remove nodeclaims, as well as evict pods. For a full list of the permissions, see the RBAC rules in the helm chart template. Karpenter also has AWS IAM permissions to create instances with IAM roles.

* [aggregate-clusterrole.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/aggregate-clusterrole.yaml)
* [clusterrole-core.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/clusterrole-core.yaml)
* [clusterrole.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/clusterrole.yaml)
* [rolebinding.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/rolebinding.yaml)
* [role.yaml](https://github.com/aws/karpenter/blob{{< githubRelRef >}}charts/karpenter/templates/role.yaml)

## Assumptions

|   Category   |                                                                                                       Assumption                                                                                                      |                                                                                                              Comment                                                                                                             |
|:------------:|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|:--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------:|
| Generic      | The Karpenter pod is operated on a node in the cluster, and uses a Service Account or Pod Identity for authentication to the Kubernetes API                                                                           | Cluster Operators may want to isolate the node running the Karpenter pod to a system-pool of nodes to mitigate the possibility of container breakout with Karpenter’s permissions.                                               |
| Generic      | Application Owners do not have any Kubernetes permissions to manage Karpenter running in the cluster (The deployment, pods, clusterrole, etc)                                                                         |                                                                                                                                                                                                                                  |
| Generic      | Restrictions on the fields of pods Application Owners can create are out of scope.                                                                                                                                    | Cluster Operators can use policy frameworks to enforce restrictions on Pod capabilities                                                                                                                                          |
| Generic      | No sensitive data is included in non-Secret resources in the Kubernetes API. The Karpenter controller has the ability to list all pods, nodes, deployments, and many other pod-controller and storage resource types. | Karpenter does not have permission to list/watch cluster-wide ConfigMaps or Secrets                                                                                                                                              |
| Generic      | Karpenter has permissions to create, modify, and delete nodes from the cluster, and evict any pod.                                                                                                                    | Cluster Operators running applications with varying security profiles in the same cluster may want to configure dedicated nodes and scheduling rules for Karpenter to mitigate potential container escapes from other containers |
| AWS-Specific | The Karpenter IAM policy is encoded in the GitHub repo. Any additional permissions possibly granted to that role by the administrator are out of scope                                                                |                                                                                                                                                                                                                                  |
| AWS-Specific | The Karpenter pod uses IRSA or Pod Identity for AWS credentials                                                                                                                                                       | Setup of IRSA and Pod Identity is out of scope for this document                                                                                                                                                                 |

## Threats and Mitigations 

### Runaway Scaling 

#### NodeClaims

**Threat:** Any user who has permission to CREATE `NodeClaim` permission on the Kubernetes API server has implicit permission to launch new EC2 instances, even if they don’t have the IAM permission to launch the instances themselves. This can result in exhaustion of resources quotas possibly blocking other AWS services from operating within the account.

**Mitigation:** CREATE `NodeClaim` is treated as a privileged permission, users who would not otherwise have permission to launch instances. By default, only the Cluster Operator has access to this resource. If the Cluster Operator grants another user permission, this is the user’s responsibility.

#### Pods

**Threat:** Application Owners are able to leverage Karpenter to launch new instances in response to their pending pods, even if they don’t have permissions to launch instances themselves. This can result in exhaustion of resources quotas  (instance vcpu limits, IP addresses, etc) potentially blocking other service or workloads within the account the account. 

**Mitigation:** Karpenter enables Cluster Operators to configure maximum resources to limit resource spend on `NodePools`.  We restrict instance types that we launch to only those that will not exceed the specified limit.  If there are no such instance types available to the `NodePool`, we will not launch any nodes.


### Privilege Escalation

#### Deleting/Creating Instances through NodeClaim Updates

**Threat:** Any user who has permission to Update `NodeClaims` on the Kubernetes API server has implicit permission to launch new EC2 instances, even if they don’t have the IAM permission to launch the instances themselves. Karpenter uses [NodeClaim annotations](https://karpenter.sh/docs/concepts/disruption/#drift) resource as sources of truth for detecting Drift and mapping to an EC2 Instance. A user who has access to edit fields on `NodeClaim` annotation may not have `RunInstances/TerminateInstances` permission, meaning that a user who has permission to Create or edit a `NodeClaim` annotation  can modify the EC2 instance it refers to, escalating their privilege to orchestrate Karpenter to terminate an existing instance. 

**Mitigation:** Karpenter recommends granting Write permissions to `NodeClaim` to only those Kubernetes RBAC entities which should be able to create/delete EC2 instances. Generally, users of any Kubernetes RBAC entity that has Write permissions on Nodes should have the same Write permissions on `NodeClaim` Although user don’t need to modify `NodeClaims`. 

#### Deleting/Creating EC2 Instances through NodePool and EC2NodeClass Updates

**Threat:** Karpenter discovers SecurityGroups, Subnets, and AMIs on the EC2NodeClass for detecting Drift. A user who has access to modify fields on NodePool/EC2NodeClass may not also have `RunInstances/TerminateInstances` permission, meaning that a user who has permission to edit these custom resources can escalate their privilege to orchestrate Karpenter to creating and terminate an instances.

**Mitigation:** Karpenter recommends granting the permission to create and modify NodePool and `EC2NodeClass` to only those Kubernetes RBAC entities which should be able to create/delete EC2 instances. The use of Karpenter `NodePool` is analogous to giving the RBAC entity all the IAM permissions that Karpenter holds.

#### Deleting/Creating EC2 Instances through Updating Tags

**Threat:** Karpenter discovers SecurityGroups, Subnets, and AMIs on the EC2NodeClass for detecting Drift. A user who has the ability to modify tags on SecurityGroups, Subnets, and AMIs can trigger drift  may not have `RunInstances/TerminateInstances` permission meaning a user who has permission to add/edit tags on SecurityGroups, Subnets, and AMIs can escalate their privilege to orchestrate Karpenter to creating and terminate an instances.

**Mitigation:** Karpenter recommends that granting permission to editing tags that are used by Karpenter to create and modify tags on SecurityGroups, Subnets, and AMIs to only entities with create/delete EC2 instances.

#### Karpenter launches an EC2 instance using an unintended AMI

**Threat:** Application Owners can create `EC2NodeClass` that refer to an AMI by name. A user can launch with a public AMI with the same name that is different than AMI interned by the user.

**Mitigation:** When selecting AMIs by name, Karpenter defaults to adding an ownership filter of self,amazon so AMI images external to the account are not used.

#### Karpenter can be used to create or terminate EC2 instances outside the cluster

**Threat:** EC2 instances can exist in an AWS account outside of the Kubernetes cluster. An actor who obtains control of the Karpenter pod’s IAM role may be able to create or terminate EC2 instances not part of the Kubernetes cluster managed by Karpenter.

**Mitigation:** Karpenter creates instances with tags, several of which are enforced in the IAM policy granted to the Karpenter IAM role that restrict the instances Karpenter can terminate. One tag requires that the instance was provisioned by a Karpenter controller (`karpenter.sh/nodepool`), another tag can include a cluster name to mitigate any termination between two clusters with Karpenter in the same account `kubernetes.io/cluster/${CLUSTER_NAME}`. Cluster Operators also can restrict the region to prevent two clusters in the same account with the same name in different regions. The IAM permissions provided to customers restrict region out of the box by default. Additionally, Karpenter does not allow tags to be created or modified on instances unowned by Karpenter after creation, except for the `Name` and `karpenter.sh/nodeclaim` tags. Though these tags can be changed after instance creation, `aws:ResourceTag` conditions enforce that the Karpenter controller is only able to change these tags on instances that it already owns, enforced through the `karpenter.sh/nodepool` and `kubernetes.io/cluster/${CLUSTER_NAME}` tags.

#### Karpenter can orchestrate the creation/deletion of IAM Instance Profiles it doesn’t own

**Threat:** Karpenter has permission to create/update/delete instance profiles as part of its controller permissions to ensure that it can auto-generate instance profiles when `EC2NodeClasses` are created. An actor who obtains control of the Karpenter pod’s IAM role may delete instance profiles not owned by Karpenter, causing workload disruption to other instances using the profile in the account.

**Mitigation:** Karpenter’s controller permissions enforce that it creates instance profiles with tags which provide ownership information. These tags include:

* `karpenter.sh/managed-by`
* `kubernetes.io/cluster/${CLUSTER_NAME}`
* `karpenter.k8s.aws/ec2nodeclass`
* `topology.kubernetes.io/region`

Source: https://karpenter.sh/docs/reference/cloudformation/#allowscopedinstanceprofilecreationactions

These tags ensure that instance profiles created by Karpenter in the account are unique to that cluster. Karpenter’s controller permissions only allow it to act on instance profiles that contain these tags which match the cluster information.

#### Operator and developer have different permissions

**Threat:** When installing Karpenter, cluster operators define an [IAM Role](https://karpenter.sh/docs/reference/cloudformation/#controller-authorization) for Karpenter to run under. Additionally, cluster operators must provide an [Instance IAM Role](https://karpenter.sh/docs/reference/cloudformation/#node-authorization) or Instance Profile for EC2 Instance launched by Karpenter. If the developer had permission to exec into the Karpenter pod, they would elevate their permissions to that of the IAM role for Karpenter.

**Mitigation:** Karpenter runs in a separate namespace [(currently recommended to run under kube-system namespace)](https://karpenter.sh/docs/getting-started/getting-started-with-karpenter/#preventing-apiserver-request-throttling) , which enables cluster operators to restrict access using Kubernetes RBAC. 

#### Undesired Disruption Through NodePool Access

Problem: If the user is able to modify the `NodePool` for a given budget, they can edit the budgets to change the resulting behavior. This could either terminate nodes when the user doesn’t want it, or block Karpenter to terminate and patch nodes when a user does. 

Mitigation: Access to the `NodePool` is treated as a privileged permission. If an Application Owners had access to the `NodePool`, they could modify or delete everything about the existing nodes in the cluster. By default, only the Cluster Operator has access to this resource. If the Cluster Operator grants another user permission, this is the user’s responsibility. Karpenter also emits a metric for the current allowed disruption that users can monitor in case their `NodePools` are modified. Karpenter will also emit an event for the `NodePool` if disruptions are allowed. 

#### Launching EC2 instances with IAM roles not intended for Karpenter nodes

**Threat:** Many IAM roles in an AWS account may trust the EC2 service principal. IAM administrators must grant the `iam:PassRole` permission to `IAM principals` to allow those principals in the account to launch instances with specific roles. A Cluster Operator attempts to create an `EC2NodeClass` with an IAM role not intended for Karpenter

**Mitigation:** Cluster Operators must enumerate the roles in the resource section of the IAM policy granted to the Karpenter role for the `iam:PassRole` action. Karpenter will fail to generate an instance profile if role that is specified in the `spec.role` section of the `EC2NodeClass` is not enumerated in the `iam:PassRole` permission.

#### Execution of Arbitrary UserData

**Threat:** Karpenter allows users to specify custom UserData as part of the `EC2NodeClass` resource. This UserData may take two shapes: a MIME multi-part archive or a NodeConfig yaml object. Karpenter does not perform any validation on this input, other than parsing the MIME multi-part archive if a header is detected. This custom UserData is then merged with UserData generated by Karpenter and is included in the Launch Template used to launch Karpenter managed nodes. Users with EDIT permissions on `EC2NodeClass` are able arbitrarily execute code on nodes launched by Karpenter.

**Mitigation:** Access to `EC2NodeClass` is treated as a privileged permission. Access to the `EC2NodeClass` enables a user to not only specify custom UserData but also modify the IAM role used by the instance profile generated for the node, select the Node’s subnets and security groups, and other privileged settings. By default, only the Cluster Admin has access to this resource. If the Cluster Admin grants another user permission, this is the Cluster Admin’s responsibility. 

### Denial of Service

#### Repeated DescribeImages Call to the EC2 API

**Threat:** Any user that has CREATE or EDIT EC2NodeClass permission can make read API calls. An actor could create add many [AMISelectorTerms](https://karpenter.sh/docs/concepts/nodeclasses/#specamiselectorterms), using tags or name , which could result in Karpenter making repeated calls to EC2. One set of tags can potentially make 70 `DescribeImages` call due to the filtering and pagination operation of the EC2 APIs. This is due to the nature of tag based [pagination](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html#api-pagination) operations for DescribeImages API. 

**Mitigation:** Access to the `EC2NodeClass` is treated as a privileged permission. If an Application Owners had access to the `EC2NodeClass`, they could modify to cause Karpenter to make AWS API calls. By default, only the Cluster Operator has access to this resource. If the Cluster Operator grants another user permission, this is the user’s responsibility.
