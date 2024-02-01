# Ami Minimum Age

This document describes a design for a new feature to allow users to specify a minimum age for AMIs.

(a @jonathan-innis's idea, see [this comment](https://github.com/aws/karpenter-provider-aws/issues/5382#issuecomment-1868068193))

## Background

Karpenter allows users to specify TTL for nodes, which configures the maximum age of nodes in the cluster. 

This is useful to ensure that nodes are replaced periodically, which helps to ensure that the cluster is running on the latest AMIs available, which simplifies and automates the security patching process.

However, a recent issue with a released AMI (see [this issue](https://github.com/awslabs/amazon-eks-ami/issues/1551)) has highlighted that it would be useful to be able to specify a minimum age for AMIs, to ensure that nodes are not replaced with an AMI that is too new.

## Proposed solution

Add a new field `amiMinimumAge` to the `EC2NodeClass`

```
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: my-node-class
spec:
  amiFamily: AL2
  amiMinimumAge: 2w
```

and modify the logic that selects the AMI to ensure that the AMI is at least `amiMinimumAge` old.

The parameter applies to AMIs resolved via:
- `amiFamily`
- `amiSelectorTerms`

### AmiSelectorTerms and AmiMinimumAge

If `amiSelectorTerms` are specified, the AMIs candidates are currently resolved querying the AWS EC2 API for AMIs, applying a filter to match the selector terms.

Once the images are retrieved from the API, the images younger than `amiMinimumAge` are filtered out.

### AmiFamily and AmiMinimumAge

If no `amiSelectorTerms` are specified, the AMIs candidates are currently resolved querying the SSM Parameter Store for the recommended EKS AMI matching the `amiFamily`,
and then querying the AWS EC2 API for the AMI details (by specific `image-id`s).

One solution would be switching to a broader query, so, instead of querying for the recommended AMI, we query for all the AMIs matching the `amiFamily` and then filter out the images younger than `amiMinimumAge`. 

for example, AL2 family, amd64 architecture:

```
{
    Query: fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2", version),
    Subpath: "recommended/image_id"
    Requirements: scheduling.NewRequirements(
        scheduling.NewRequirement(v1.LabelArchStable, v1.NodeSelectorOpIn, corev1beta1.ArchitectureAmd64),
        scheduling.NewRequirement(v1beta1.LabelInstanceGPUCount, v1.NodeSelectorOpDoesNotExist),
        scheduling.NewRequirement(v1beta1.LabelInstanceAcceleratorCount, v1.NodeSelectorOpDoesNotExist),
    ),
}
```

We can then query the EC2 API for the AMIs matching the `image-id`s returned by the SSM query, and look for the recommended `image-ids`:
- if the image with the recommended `image-id` is older than `amiMinimumAge`, we keep it as candidate
- if the image with the recommended `image-id` is younger than `amiMinimumAge`, we look for the next image in the list that matches the `imageLocation` pattern and is older than `amiMinimumAge`