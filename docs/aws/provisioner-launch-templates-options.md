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

### Launch Templates and Architecture

At present, Launch Templates must specify an AMI-ID (ImageId)
([docs](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RequestLaunchTemplateData.html)).
Although the `ImageId` field is not, technically, required, since
CreateFleet does not allow overriding the `ImageId`, it effectively is
required. Since AMIs are architecture-specific, this means that Launch
Templates are, transitively, arch specific.
