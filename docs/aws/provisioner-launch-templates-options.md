# AWS Provisioner Launch Template Options

## Intro

This document presents some options for how the AWS-specific portions
of the Provisioner could handle [Launch
Templates](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-launch-templates.html).

Presently, the Provisioner has the following shape:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha1
kind: Provisioner
metadata:
  name:
spec:
  cluster:
    name:
    caBundle:
    endpoint:
  architecture:
  taints:
    - key:
      effect:
  zones:
	- 
  instanceTypes:
    -
  ttlSeconds:
  # Labels will be applied to every node launched by the Provisioner unless
  # overriden by pod node selectors. Well known labels control provisioning
  # behavior. Additional labels may be supported by your cloudprovider.
  labels:
    kubernetes.amazonaws.com/launchTemplateId:
    kubernetes.amazonaws.com/launchTemplateVersion:
    kubernetes.amazonaws.com/capacityType:
```

As of Mar 2021, support for these labels is not yet implemented, so
before doing this document presents some alternatives. The next
section outlines why we might want to do this.

## Potential Issues

### Label Naming Convention

The [AWS Controllers for Kubernetes
(ACK)](https://github.com/aws-controllers-k8s/community) project is
using the label naming convention of `*.k8s.aws/*`. Should the AWS
portion of this project follow that convention?

Another consideration is that, regardless of which suffix is
ultimately chosen, should the node labels look like
`node.<suffix>/something` rather than `<suffix>/something`?

Lastly, a casual survey of other widely-used Kubernetes labels would
suggest that using dashes is more common than camel case, so the
portion following the `/` should most likely look like
`launch-template-id`.

### Launch Templates and Architecture

At present, Launch Templates must specify an AMI-ID (ImageId)
([docs](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestLaunchTemplateData.html)).
Although the `ImageId` field is not, technically, required, since
CreateFleet does not allow overriding the `ImageId`, it effectively is
required. Since AMIs are architecture-specific, this means that Launch
Templates are, transitively, architecture-specific as well.

One problem is that this might be confusing due to the presence of the
`architecture` field in the spec. The implication to a user, if they
do not specify an architecture, is that the provisioner can handle all
architectures. But if they specify a launch template, their
provisioner is now architecture-specific.

Another potential issue is that the launch template label could
conflict with the architecture. For example, imagine a customer
specified:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha1
kind: Provisioner
spec:
  architecture: arm64
  labels:
    kubernetes.amazonaws.com/launchTemplateId: id-of-x86_64-based-ami
```

In this case, the provisioner will only launch ARM-based instances,
which will fail because the AMI is for the other CPU architeture. This
might not come up often enough, in practice, to be worth changing the
design. But it is worth discussion.

Similarly, with this scheme, there is no way to configure a
provisioner to support two different launch templates. If a user
wanted to support both ARM and x86 in the same cluster, they'd have to
create two separate provisioners.

The provisioner does not allow the customer to specify a
`kubernetes.io/arch` label in the `labels` section, so there is no
risk of a conflict between that and the launch template.

# Solutions

## Label Naming

It seems like we should standardize on a label naming convention, and
since ACK is using it, and it's short, we should use `k8s.aws` as the
root suffix going forward. (Obviously while maintaining
backwards-compatibility with any extant labels.)

Furthermore, in keeping with apparent Kubernetes
[convention](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesioinstance-type),
we should use `node.k8s.aws/` as the prefix for any node labels we
choose to use.

Lastly, in keeping with Kubernetes conventions (like
`kubernetes.io/service-name`), we should use `lower-case-with-hypens`,
not `camelCase` for words following the `/`.

One paritial solution to this would be to remove the ability to
specify the `architecture` in the provisioner at all. However, this
does not seem like a good idea because the provisioner is not specific
to EC2, and therefore the specific behavior of provider behavior
(launch templates) doesn't seem like it merits that kind of change.

Another solution would be to allow the user to specify architecture
specific labels:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha1
kind: Provisioner
spec:
  labels:
    arm64:
	  kubernetes.amazonaws.com/launchTemplateId: id-of-arm64-based-ami
	x86_64:
	  kubernetes.amazonaws.com/launchTemplateId: id-of-x86_64-based-ami
	  
```




