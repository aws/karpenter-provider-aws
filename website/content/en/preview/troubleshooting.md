---
title: "Troubleshooting"
linkTitle: "Troubleshooting"
weight: 100
---

## Node NotReady

There are many reasons that a node can fail to join the cluster.
- Permissions
- Security Groups
- Networking

The easiest way to start debugging is to connect to the instance
```sh
# List the nodes managed by Karpenter
kubectl get node -l karpenter.sh/provisioner-name
# Extract the instance ID
INSTANCE_ID=$(kubectl get node -l karpenter.sh/provisioner-name -ojson | jq -r ".items[0].spec.providerID" | cut -d \/ -f5)
# Connect to the instance
aws ssm start-session --target $INSTANCE_ID
# Check Kubelet logs
sudo journalctl -u kubelet
```

## Missing Service Linked Role
Unless your AWS account has already onboarded to EC2 Spot, you will need to create the service linked role to avoid `ServiceLinkedRoleCreationNotPermitted`.
```
AuthFailure.ServiceLinkedRoleCreationNotPermitted: The provided credentials do not have permission to create the service-linked role for EC2 Spot Instances
```
This can be resolved by creating the [Service Linked Role](https://docs.aws.amazon.com/batch/latest/userguide/spot_fleet_IAM_role.html).
```
aws iam create-service-linked-role --aws-service-name spot.amazonaws.com
```

## Unable to delete nodes after uninstalling Karpenter
Karpenter adds a [finalizer](https://github.com/aws/karpenter/pull/466) to nodes that it provisions to support graceful node termination. If Karpenter is uninstalled, these finalizers will cause the API Server to block deletion until the finalizers are removed.

You can fix this by patching the node objects:
- `kubectl edit node <node_name>` and remove the line that says `karpenter.sh/termination` in the finalizers field.
- Run the following script that gets all nodes with the finalizer and removes all the finalizers from those nodes.
   - NOTE: this will remove ALL finalizers from nodes with the karpenter finalizer.
```{bash}
kubectl get nodes -ojsonpath='{range .items[*].metadata}{@.name}:{@.finalizers}{"\n"}' | grep "karpenter.sh/termination" | cut -d ':' -f 1 | xargs kubectl patch node --type='json' -p='[{"op": "remove", "path": "/metadata/finalizers"}]'
```

## Nil issues with Karpenter reallocation
If you create a Karpenter Provisioner while the webhook to default it is unavailable, it's possible to get unintentionally nil fields. [Related Issue](https://github.com/aws/karpenter/issues/463).

   You may see some logs like this.
```{bash}
github.com/aws/karpenter/pkg/controllers/provisioning/v1alpha1/reallocation/utilization.go:84 +0x688
github.com/aws/karpenter/pkg/controllers/provisioning/v1alpha1/reallocation.(*Controller).Reconcile(0xc000b004c0, 0x23354c0, 0xc000e209f0, 0x235e640, 0xc002566c40, 0x200c786, 0x5, 0xc00259c1b0, 0x1)        github.com/aws/karpenter/pkg/controllers/provisioning/v1alpha1/reallocation/controller.go:72 +0x65
github.com/aws/karpenter/pkg/controllers.(*GenericController).Reconcile(0xc000b00720, 0x23354c0, 0xc000e209f0, 0xc001db9be0, 0x7, 0xc001db9bd0, 0x7, 0xc000e209f0, 0x7fc864172d20, 0xc0000be2a0, ...)
```
This is fixed in Karpenter v0.2.7+. Reinstall Karpenter on the latest version.

## Nodes stuck in pending and not running the kubelet due to outdated CNI
If you have an EC2 instance get launched that is stuck in pending and ultimately not running the kubelet, you may see a message like this in your `/var/log/user-data.log`:

> No entry for c6i.xlarge in /etc/eks/eni-max-pods.txt

This means that your CNI plugin is out of date. You can find instructions on how to update your plugin [here](https://docs.aws.amazon.com/eks/latest/userguide/managing-vpc-cni.html).

## Failed calling webhook "defaulting.webhook.provisioners.karpenter.sh"

If you are not able to create a provisioner due to `Error from server (InternalError): error when creating "provisioner.yaml": Internal error occurred: failed calling webhook "defaulting.webhook.provisioners.karpenter.sh": Post "https://karpenter-webhook.karpenter.svc:443/default-resource?timeout=10s": context deadline exceeded`

Verify that webhook is running
```text
kubectl get po -A -l karpenter=webhook
NAMESPACE   NAME                                READY   STATUS    RESTARTS   AGE
karpenter   karpenter-webhook-d644c7567-cdc4d   1/1     Running   0          37m
karpenter   karpenter-webhook-d644c7567-dn9xw   1/1     Running   0          37m
```

Webhook service has endpoints assigned to it
```text
kubectl get ep -A -l app.kubernetes.io/component=karpenter
NAMESPACE   NAME                ENDPOINTS                        AGE
karpenter   karpenter-metrics   10.0.13.104:8080                 38m
karpenter   karpenter-webhook   10.0.1.25:8443,10.0.30.46:8443   38m
```

Your security groups are not blocking you from reaching your webhook.

This is especially relevant if you have used `terraform-eks-module` version `>=18` since that version changed its security
approach, and now it's much more restrictive.

## DaemonSets can result in deployment failures

For Karpenter versions 0.5.3 and earlier, DaemonSets were not properly considered when provisioning nodes.
This sometimes caused nodes to be deployed that could not meet the needs of the requested DaemonSets and workloads.
This issue no longer occurs after Karpenter version 0.5.3 (see [PR #1155](https://github.com/aws/karpenter/pull/1155)).

If you are using a pre-0.5.3 version of Karpenter, one workaround is to set your provisioner to only use larger instance types that you know will be big enough for the DaemonSet and the workload.
For more information, see [Issue #1084](https://github.com/aws/karpenter/issues/1084).
Examples of this behavior are included in [Issue #1180](https://github.com/aws/karpenter/issues/1180).

## Unspecified resource requests cause scheduling/bin-pack failures

Not using the Kubernetes [LimitRanges](https://kubernetes.io/docs/concepts/policy/limit-range/) feature to enforce minimum resource request sizes will allow pods with very low or non-existent resource requests to be scheduled.
This can cause issues as Karpenter bin-packs pods based on the resource requests.

If the resource requests do not reflect the actual resource usage of the pod, Karpenter will place too many of these pods onto the same node resulting in the pods getting CPU throttled or terminated due to the OOM killer.
This behavior is not unique to Karpenter and can also occur with the standard `kube-scheduler` with pods that don't have accurate resource requests.

To prevent this, you can set LimitRanges on pod deployments on a per-namespace basis.
See the Karpenter [Best Practices Guide](https://aws.github.io/aws-eks-best-practices/karpenter/#use-limitranges-to-configure-defaults-for-resource-requests-and-limits) for further information on the use of LimitRanges.

## Missing subnetSelector and securityGroupSelector tags causes provisioning failures

Starting with Karpenter v0.5.5, if you are using Karpenter-generated launch template, provisioners require that [subnetSelector]({{<ref "./aws/provisioning/#subnetselector" >}}) and [securityGroupSelector]({{<ref "./aws/provisioning/#securitygroupselector" >}}) tags be set to match your cluster.
The [Provisioner](./getting-started/getting-started-with-eksctl/#provisioner) section in the Karpenter Getting Started Guide uses the following example:

```text
provider:
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
```
To check your subnet and security group selectors, type the following:

```bash
aws ec2 describe-subnets --filters Name=tag:karpenter.sh/discovery,Values=${CLUSTER_NAME}
```
*Returns subnets matching the selector*

```bash
aws ec2 describe-security-groups --filters Name=tag:karpenter.sh/discovery,Values=${CLUSTER_NAME}
```
*Returns security groups matching the selector*

Provisioners created without those tags and run in more recent Karpenter versions will fail with this message when you try to run the provisioner:

```text
 field(s): spec.provider.securityGroupSelector, spec.provider.subnetSelector
```
If you are providing a [custom launch template]({{<ref "./aws/launch-templates" >}}), specifiying a `subnetSelector` is still required.
However, specifying a `securityGroupSelector` will cause a validation error.

## Reapply credentials with Terraform connection failure

If you are using the [Getting Started with Terraform]({{<ref "./getting-started/getting-started-with-terraform/" >}}) instructions and you see the following error:

```text
63: resource "kubernetes_config_map" "aws_auth" {
Error: Post "http://localhost/api/v1/namespaces/kube-system/configmaps": dial tcp 127.0.0.1:80: connect: connection refused
with module.eks.kubernetes_config_map.aws_auth[0],
on .terraform/modules/eks/aws_auth.tf line 63, in resource "kubernetes_config_map" "aws_auth":
```

You can fix the problem by exporting your kubeconfig credentials:

```text
export KUBECONFIG=${PWD}/kubeconfig_${CLUSTER_NAME}
export KUBE_CONFIG_PATH=$KUBECONFIG
```

or you could run the following aws command:

```text
aws eks --region  $AWS_DEFAULT_REGION  update-kubeconfig --name $CLUSTER_NAME
```

Then rerun:

```text
terraform apply -var cluster_name=$CLUSTER_NAME
```

## Terraform fails to create instance profile when name is too long

In the Getting Started with Terraform instructions to [Configure the KarpenterNode IAM Role]({{<ref "./getting-started/getting-started-with-terraform/#configure-the-karpenternode-iam-role" >}}), the name assigned to the aws_iam_instance_profile cannot exceed 38 characters. If it does, it will fail with a message similar to:

```text
Error: expected length of name_prefix to be in the range (1 - 38), got with module.eks.aws_iam_role.cluster[0],
on .terraform/modules/eks/main.tf line 131, in resource "aws_iam_role" "cluster":
131: name_prefix = var.cluster_iam_role_name != "" ? null : var.cluster_name
```

Note that it can be easy to run over the 38-character limit considering that the example includes KarpenterNodeInstanceProfile- (29 characters) and -karpenter-demo (15 characters).
That leaves only four characters for your user name.
You can reduce the number of characters consumed by changing `KarpenterNodeInstanceProfile-` to something like `KarpenterNode-`.

## Karpenter Role names exceeding 64-character limit

If you use a tool such as AWS CDK to generate your Kubernetes cluster name, when you add Karpenter to your cluster you could end up with a cluster name that is too long to incorporate into your KarpenterNodeRole name (which is limited to 64 characters).

Node role names for Karpenter are created in the form `KarpenterNodeRole-${Cluster_Name}` in the [Create the KarpenterNode IAM Role]({{<ref "./getting-started/getting-started-with-eksctl/#create-the-karpenternode-iam-role" >}}) section of the getting started guide.
If a long cluster name causes the Karpenter node role name to exceed 64 characters, creating that object will fail.

Keep in mind that `KarpenterNodeRole-` is just a recommendation from the getting started guide.
Instead using of the eksctl role, you can shorten the name to anything you like, as long as it has the right permissions.

## Node terminates before ready on failed encrypted EBS volume
If you are using a custom launch template and an encrypted EBS volume, the IAM principal launching the node may not have sufficient permissions to use the KMS custom managed key (CMK) for the EC2 EBS root volume.
As a result, the node terminates almost immediately upon creation.
To correct this, you can use the approach that AWS EBS uses, which avoids adding particular roles to the KMS policy:

```text
[
{
"Sid": "Allow access through EBS for all principals in the account that are authorized to use EBS",
"Effect": "Allow",
"Principal": {
"AWS": ""
},
"Action": [
"kms:Encrypt",
"kms:Decrypt",
"kms:ReEncrypt",
"kms:GenerateDataKey*",
"kms:CreateGrant",
"kms:DescribeKey"
],
"Resource": "",
"Condition": {
"StringEquals": {
"kms:ViaService": "ec2.eu-west-1.amazonaws.com",
"kms:CallerAccount": "618668373247"
}
}
},
{
"Sid": "Allow direct access to key metadata to the account",
"Effect": "Allow",
"Principal": {
"AWS": "arn:aws:iam::618668373247:root"
},
"Action": [
"kms:Describe",
"kms:Get*",
"kms:List*",
"kms:RevokeGrant"
],
"Resource": "*"
}
]
```
