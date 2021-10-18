# Troubleshooting

## Known Problems + Solutions


### Missing Service Linked Role
Unless your AWS account has already onboarded to EC2 Spot, you will need to create the service linked role to avoid `ServiceLinkedRoleCreationNotPermitted`.
```
karpenter-controller-5f9869988f-czcvj manager 2021-10-18T20:20:54.437Z	ERROR	controller.controller.Allocation	Reconciler error	{"commit": "8dd6b65", "reconciler group": "karpenter.sh", "reconciler kind": "Provisioner", "name": "default", "namespace": "", "error": "launching 1 instance(s), with fleet error(s), AuthFailure.ServiceLinkedRoleCreationNotPermitted: The provided credentials do not have permission to create the service-linked role for EC2 Spot Instances."}
```
This can be resolved by creating the [Service Linked Role](https://docs.aws.amazon.com/batch/latest/userguide/spot_fleet_IAM_role.html).
```
aws iam create-service-linked-role --aws-service-name spot.amazonaws.com
```

### Unable to delete nodes after uninstalling Karpenter
Karpenter adds a [finalizer](https://github.com/awslabs/karpenter/pull/466) to nodes that it provisions to support graceful node termination. If Karpenter is uninstalled, these finalizers will cause the API Server to block deletion until the finalizers are removed.

You can fix this by patching the node objects:
- `kubectl edit node <node_name>` and remove the line that says `karpenter.sh/termination` in the finalizers field.
- Run the following script that gets all nodes with the finalizer and removes all the finalizers from those nodes.
   - NOTE: this will remove ALL finalizers from nodes with the karpenter finalizer.
```{bash}
kubectl get nodes -ojsonpath='{range .items[*].metadata}{@.name}:{@.finalizers}{"\n"}' | grep "karpenter.sh/termination" | cut -d ':' -f 1 | xargs kubectl patch node --type='json' -p='[{"op": "remove", "path": "/metadata/finalizers"}]'
```

### Nil issues with Karpenter reallocation
If you create a Karpenter Provisioner while the webhook to default it is unavailable, it's possible to get unintentionally nil fields. [Related Issue](https://github.com/awslabs/karpenter/issues/463).

   You may see some logs like this.
```{bash}
github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/reallocation/utilization.go:84 +0x688
github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/reallocation.(*Controller).Reconcile(0xc000b004c0, 0x23354c0, 0xc000e209f0, 0x235e640, 0xc002566c40, 0x200c786, 0x5, 0xc00259c1b0, 0x1)        github.com/awslabs/karpenter/pkg/controllers/provisioning/v1alpha1/reallocation/controller.go:72 +0x65
github.com/awslabs/karpenter/pkg/controllers.(*GenericController).Reconcile(0xc000b00720, 0x23354c0, 0xc000e209f0, 0xc001db9be0, 0x7, 0xc001db9bd0, 0x7, 0xc000e209f0, 0x7fc864172d20, 0xc0000be2a0, ...)
```
This is fixed in Karpenter v0.2.7+. Reinstall Karpenter on the latest version.
