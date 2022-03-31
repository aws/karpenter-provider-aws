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

For Karpenter versions 0.5.3 and earlier, Daemonsets were not properly considered when provisioning nodes.
This sometimes caused nodes to be deployed that could not meet the needs of the requested Daemonsets.
The result could be log messages like the following:

```text
Excluding instance type r3.8xlarge because there are not enough resources for daemons {"commit": "7e79a67", "provisioner": "default"}
```

One workaround is to set your provisioner to only use larger instance types.
For more information, see [Issue #1084](https://github.com/aws/karpenter/issues/1084).
Examples of this behavior are included in [Issue #1180](https://github.com/aws/karpenter/issues/1180).
This issue was addressed in later Karpenter releases by [PR #1155](https://github.com/aws/karpenter/pull/1155).

## Unspecified resource requests cause scheduling/bin-pack failures

Not using the Kubernetes [LimitRanges](https://kubernetes.io/docs/concepts/policy/limit-range/) feature to enforce minimum resource request sizes will allow pods with very low or non-existent resource requests to be scheduled.
This can cause issues as Karpenter bin-packs pods based on the resource requests.

If the resource requests do not reflect the actual resource usage of the pod, Karpenter will place too many of these pods onto the same node resulting in the pods getting CPU throttled or terminated due to the OOM killer.
This behavior is not unique to Karpenter and can also occur with the standard `kube-scheduler` with pods that don't have accurate resource requests.

To prevent this, you can set LimitRanges on pod deployments on a per-namespace basis.
See the Karpenter [Best Practices Guide](https://aws.github.io/aws-eks-best-practices/karpenter/#use-limitranges-to-configure-defaults-for-resource-requests-and-limits) for further information on the use of LimitRanges.

## Missing subnetSelector and securityGroupSelector tags causes provisioning failures

Starting with Karpenter v0.5.5, provisioners require [subnetSelector](./aws/provisioning/#subnetselector) and [securityGroupSelector](./aws/provisioning/#securitygroupselector) tags be set to match your cluster.
The [Provisioner](./getting-started/getting-started-with-eksctl/#provisioner) example in the Karpenter Getting Started Guide uses the following:

```text
provider:
    subnetSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
    securityGroupSelector:
      karpenter.sh/discovery: ${CLUSTER_NAME}
```

Provisioners created without those tags and run in more recent Karpenter versions will fail with this message when you try to run the provisioner:

```text
 field(s): spec.provider.securityGroupSelector, spec.provider.subnetSelector
```
