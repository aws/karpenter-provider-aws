# Placement Group Configuration

## Goals
* Allowing a user to configure EC2 Placement Group strategies from an `EC2NodeClass` resource
* Allow the Placement Group strategies to be developed independently from one another

## Background
At present, there is no way to configure an [EC2 Placement Group](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/placement-groups.html) with Karpenter. The addition of this feature will expose an important feature of EC2 to Karpenter users, expanding the set of use-cases the tool is appropriate for.

## Proposed Design
To enable the configuration of EC2 Placement Groups with Karpenter, I propose the addition of a `.spec.placementStrategy` value in the `karpenter.k8s.aws/v1beta1/EC2NodeClass` resource. This new value will have three child fields --  `cluster`, `partition`, and `spread` -- only one of which can be enabled for a given `EC2NodeClass` instance, which is in line with the EC2 API. 

Given the simple and self-contained nature of EC2 Placement Group configurations, my sense is that it makes litle sense to enable the import of preexisting Placement Group configurations. This would add implementation complexity without actually enabling additional features for the user. Instead, Karpenter should completely manage the lifecycle of EC2 Placement Group resources, tying their lifetimes to the lifetime of the `.spec.placementStrategy` value in the supervising EC2NodeClass resource.


The following YAML snippets depict how this would look in practice, including one case where the `EC2NodeClass` resource *should* be rejected by the Karpenter server.

```yaml
---
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: example-cluster
spec:
  # ...
  placementStrategy:
    cluster: { enabled: true } # No configuration ields
---
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: example-spread
spec:
  # ...
  placementStrategy:
    spread:
      enabled: true
      spreadLevel: host # constraints: rack | host
---
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: example-partition
spec:
  # ...
  placementStrategy:
    partition:
      enabled: true
      count: 2 # contraint: 1-7
---
# The following should be rejected by Karpenter, as it enables both 
# the 'partition' and 'cluster' strategies.
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: example-error
spec:
  # ...
  placementStrategy:
    partition:
      enabled: true
      count: 2
    cluster: 
      enabled: true
```
