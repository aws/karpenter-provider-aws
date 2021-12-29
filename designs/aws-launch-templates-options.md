# AWS Launch Template Options
*Authors: JacobGabrielson@*
## Intro

This document presents some options for how the AWS-specific (cloud
provider) portions of the provisioner could handle [Launch
Templates](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-launch-templates.html).

Presently, the provisioner has the following shape:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha2
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
  # overridden by pod node selectors. Well known labels control provisioning
  # behavior. Additional labels may be supported by your cloudprovider.
  labels:
    # These are AWS-specific
    kubernetes.amazonaws.com/launchTemplateId:
    kubernetes.amazonaws.com/launchTemplateVersion:
    kubernetes.amazonaws.com/capacityType:
```

As of Mar 2021, support for these labels in the AWS cloud provider is
not yet implemented, so before doing that work, this document presents
some alternatives. The next section outlines why we might want to do
this.

## Potential Issues

### Label Naming Convention

The [AWS Controllers for Kubernetes
(ACK)](https://github.com/aws-controllers-k8s/community) project is
using the label naming convention of `*.k8s.aws/*`. The AWS cloud
provider portion of this project should consider following that
convention.

Another consideration is that, regardless of which suffix is
ultimately chosen, the node labels should possibly look like
`node.<suffix>/something`, rather than `<suffix>/something`.

Lastly, a casual survey of other widely-used Kubernetes labels shows
that using dashes is more common than camel case, so the portion
following the `/` should probably look like `launch-template-id`.

### Launch Templates and Architecture

At present, EC2 launch templates must specify an AMI-ID (`ImageId`)
([docs](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestLaunchTemplateData.html)).
Although the field is not, technically, required, in practice is
actually is mandatory because
[CreateFleet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html)
does not allow
[overriding](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_FleetLaunchTemplateOverridesRequest.html)
the `ImageId`. Since AMIs are architecture-specific, this means that
launch templates are, transitively, architecture-specific as well.

One problem is that this might be confusing due to the presence of the
`architecture` field in the spec. The implication to a user, if they
do not specify `architecture`, is that the provisioner can handle all
architectures. But if they specify a launch template, their
provisioner is now architecture-specific.

Another potential issue is that the launch template label could
conflict with the architecture. For example, imagine a customer
specified:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha2
kind: Provisioner
spec:
  architecture: arm64
  labels:
    kubernetes.amazonaws.com/launchTemplateId: id-of-x86_64-lt
```

In this case, the provisioner will only launch ARM-based instances,
which will fail because the AMI is for the other CPU architecture. This
might not come up often enough, in practice, to be worth changing the
design. But it is worth discussion.

Similarly, with this scheme, there is no way to configure a
provisioner to support two different launch templates. If a user
wanted to support both ARM and x86 in the same cluster, they'd have to
create two separate provisioners.

The provisioner does not allow the customer to specify a
`kubernetes.io/arch` label in the `labels` section, so there is no
risk of a conflict between that and the launch template.

Another problem users might run into is that their pod spec might
specify a node selector for an architecture that the provisioner
doesn't support.

# Solutions

## Label Naming

It seems like we should standardize on a label naming convention, and
since ACK is using it, and it's short, we should use `k8s.aws` as the
root suffix going forward. (Obviously while maintaining
backwards-compatibility with any extant labels.)

Furthermore, in keeping with Kubernetes
[convention](https://kubernetes.io/docs/reference/labels-annotations-taints/#nodekubernetesioinstance-type),
we should use `node.k8s.aws/` as the prefix for any node labels we
choose to use.

In keeping with another Kubernetes convention (such as
`kubernetes.io/service-name`), we should use `lower-case-with-hyphens`,
not `camelCase` for words following the `/`.

Lastly, using launch template names, rather than ids, will be more
familiar to Kubernetes users and should cut down on manual work and
errors. Since the EC2 APIs that use launch templates can take either
names or ids, it seems like names are an more useful choice. In the
future we can consider adding id support as well, if there is demand
for it.

## Architecture

One partial solution to the architecture issue brought up above would
be to remove the ability to specify the `architecture` in the
provisioner at all. However, this does not seem like a good idea
because the provisioner is not specific to EC2, and therefore the
specific behavior of cloud provider behavior (such as launch
templates) doesn't seem like it merits that kind of change.

Another solution would be to allow the user to specify architecture
specific labels:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha2
kind: Provisioner
spec:
  labels:
    # applied only to arm64
    arm64:
      node.k8s.aws/launch-template-name: name-of-arm64-lt
    x86_64:
      node.k8s.aws/launch-template-name: name-of-x86_64-lt
    # applied everywhere
    other-label: other-value
```

Or:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha2
kind: Provisioner
spec:
  # applied everywhere
  labels:
     other-label: other-value
  # applied only to arm64
  arm64-labels:
     node.k8s.aws/launch-template-name: name-of-arm64-based-lt
  # applied only to x86_64
  x86_64-labels:
```

Another possibility is to add another "path element" to the label name
to make it architecture specific:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha2
kind: Provisioner
spec:
  labels:
      node.k8s.aws/launch-template-name/arm64: name-of-arm64-lt
      # or?
      node.k8s.aws/arm64/launch-template-name: name-of-arm64-lt
```

This might be very non-standard however and defy expectations. Also,
there is no guarantee that the user correctly specified `arm64`; the
launch template might still actually refer to an `x86_64` image.

Note also that the provisioner could determine (through EC2 APIs) the
architecture of the `ImageId` referred to by a launch template, and
then ignore pod specs that specify and incompatible
`kubernetes.io/arch` in a [node
selector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector).
For example, imagine the pod spec of a pending pod contains:

```yaml
apiVersion: v1
kind: Pod
spec:
  nodeSelector:
    kubernetes.io/arch: arm64
```

If a provisioner is configured as follows:

```yaml
apiVersion: provisioning.karpenter.sh/v1alpha2
kind: Provisioner
spec:
  labels:
      node.k8s.aws/launch-template-name: name-of-lt-with-x86_64-based-ami
```

Then it could ignore that pending pod since there is no way it could
possibly work. Another possibility is that it could ignore the launch
template `node.k8s.aws/launch-template-name` for that pod and revert
to the default, dynamically-generated launch template that would work
for that architecture.


## Recommendation

For now the recommendation is to support the following in provisioner
`spec.labels`:

- `node.k8s.aws/launch-template-name`: (optional) id of launch template
  and cannot be specified if `architecture`
- `node.k8s.aws/launch-template-version`: version number or `$LATEST`
  (optional, default `$LATEST`) (cannot be specified unless
  `node.k8s.aws/launch-template-name` is present
- `node.k8s.aws/capacity-type`: listed here for completeness

### The Fine Print

#### Validation

If the user specifies an incompatible `architecture` in the
provisioner spec, or incompatible `kubernetes.io/arch` in their pod
spec, then the provisioner will instead use the default launch
template for that architecture. (Note this feature may not be
implemented in the first version; in the first version the provisioner
will ignore pods with incompatible architectures).

#### Node Selectors and Capacity Pools

If two presently-unschedulable pods are defined like so (that is, one
has a launch template in a node selector and one does not):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: podA
spec:
  nodeSelector:
    node.k8s.aws/launch-template-name=x
---
apiVersion: v1
kind: Pod
metadata:
  name: podB
spec:
```

Then they will not launch on the same instance. In this case at least
two instances would launch.

#### Multiple Provisioners

While specifying a `launch-template-name` on a provisioner will limit
that provisioner to a single default launch template (pod specs can
still override by specifying a launch template), it seems like the
complexity of the other solutions that might allow multiple launch
templates on a single provisioner aren't worth the code complexity
(without more input from users).

### Pros

- Node labels are shorter
- Node labels match the ACK project
- Node labels conform to Kubernetes standard
- Could theoretically be extended in some future date by adding
  `/<arch>` into labels (assuming that would not violate
  standards/expectations)
- Simple and intuitive most of the time (that is, as long as users
  aren't expecting the provisioner to support multiple architectures
  and custom launch templates at the same time).
- Obvious problems (such as specifying both `architecture` and a
  launch template in the same provisioner spec) will be caught early,
  such as when the user runs `kubectl`.

### Cons

- If the user specifies an incompatible `architecture` in the
  provisioner spec, or incompatible `kubernetes.io/arch` in their pod
  spec, then that launch template won't be used (or, in initial
  versions of the provisioner, that pod would get ignored by
  Karpenter). While this might usually be what the user intended,
  sometimes it might be surprising.
- Users who want to specify their own launch template may be confused
  trying to figure out how to support multiple architectures in the
  same cluster (that is, they may find it difficult to figure out that
  they need two provisioners)
- This does require the (generic) provisioner validating webhook to
  "reach in" to the provider to do further validation, which is
  additional complexity (that said, it doesn't sound like this is
  unheard-of behavior in similar projects, either).

Customers can use multiple provisioners by differentiating in their
pod specs:

```yaml
apiVersion: v1
kind: Pod
spec:
  nodeSelector:
    kubernetes.io/arch: arm64
    provisioning.karpenter.sh/name: some-provisioner
```

# Open Questions

## CAPI Integration

It would be nice if the AWS cloud provider for Karpenter could
leverage and/or work smoothly with
[CAPI](https://github.com/kubernetes-sigs/cluster-api). We will
address this in a separate document.

## Launch Template Names

We could support something like:

- `node.k8s.aws/launch-template-name`: (optional) name of launch template

Names do not appear to be unique.
