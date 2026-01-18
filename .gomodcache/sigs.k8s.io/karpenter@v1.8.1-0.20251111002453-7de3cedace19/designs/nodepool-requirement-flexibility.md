# NodePool Requirement Flexibility 

**NodeClaim**: Karpenter’s representation of a Node request. A NodeClaim declaratively describes how the node should look, including instance type requirements that constrain the options that the Cloud Provider can launch with. For the AWS Cloud Provider, the instance requirements of the NodeClaim translate to the requirements of the CreateFleet request. More details can be found [here](https://github.com/aws/karpenter-provider-aws/blob/main/designs/node-ownership.md).

**Preflight NodeClaim:** Karpenter’s in-memory representation of a NodeClaim that is computed during a provisioning loop. This is a NodeClaim that Karpenter hasn’t created yet CD, but is planning to launch at the end of the scheduling loop. This contains the full set of instance type possibilities which change (and become more constrained) as more pods are scheduled to the NodeClaim.

## Motivation

Karpenter’s scheduling algorithm uses a well-known bin-packing algorithm ([First Fit Decreasing](https://en.wikipedia.org/wiki/First-fit-decreasing_bin_packing)) to pack as many pods onto a set of instance type options as possible. This algorithm will necessarily continue packing pods onto Preflight Nodes so long as you have at least one instance type option that can fulfill all the pod requests.

This methodology works fine for on-demand instance types, but starts to break down when you are requesting spot capacity. If you are launching spot capacity and want to ensure that you will not launch an instance that will immediately get interrupted after launch, you need to make sure that you have enough launch options in your launch request that your likelihood for launching a low-availability instance will be slim. Karpenter currently has no mechanism to ensure that today.

This RFC proposes an additional key `minValues` in the NodePool requirements block, allowing Karpenter to be aware of user-specified flexibility minimums while scheduling pods to a cluster. If Karpenter cannot meet this minimum flexibility when scheduling a pod, it will fail the scheduling loop for that NodePool, either falling back to another NodePool which meets the pod requirements or failing scheduling the pod altogether.

```
apiVersion: karpenter.sh/v1beta1
kind: NodePool
spec:
  template:
    spec:
      requirements:
        - key: karpenter.kwok.sh/instance-family
          operator: In
          values: ["c", "m", "r"]
          minValues: 2
        - key: node.kubernetes.io/instance-type
          operator: Exists
          minValues: 10
```

## Use-Cases

1. I want to configure Karpenter to enforce more flexibility when launching spot nodes, ensuring that I am constantly using enough instance types that I get a high-availability instance type
2. want to ensure that application pods scheduling to this NodePool *must* be flexible to different instance types, architectures, etc.

## Solutions

### [Recommended] Solution 1: `minValues` field in the `spec.requirements`  section of NodeClaim Template (becomes part of the template on the NodePool)

```
apiVersion: karpenter.sh/v1beta1
kind: NodePool
spec:
  template:
    spec:
      requirements:
      - key: karpenter.kwok.sh/instance-family
        operator: In
        values: ["c", "m", "r"]
        minValues: 2
      - key: node.kubernetes.io/instance-type
        operator: Exists
        minValues: 10
```

We would add a new `minValues` field to the requirements section of the NodeClaim template. By adding this section to the NodeClaim template, we inform the scheduler that it should stop continuing to bin-pack pods onto Preflight NodeClaims and should, instead, create another Preflight NodeClaim that will schedule that pod.

#### Pros/Cons

1. 👍👍 Puts all core scheduling behavior together in the same block, allowing users to easily reason about what their requirements and minimums mean
2. 👎 Piggy-backs off of a well-known core API. There’s risk that if this API changes in a way that is incompatible with this value in the future, we may have to break our `minValues` API

### Solution 2: `minValues` in NodeClass API

```
apiVersion: karpenter.sh/v1beta1
kind: EC2NodeClass
spec:
  minValues:
    node.kubernetes.io/instance-type: 10
    karpenter.kwok.sh/instance-family: 2
---
apiVersion: karpenter.sh/v1beta1
kind: NodePool
spec:
  template:
    spec:
      requirements:
      - key: karpenter.kwok.sh/instance-family
        operator: In
        values: ["c", "m", "r"]
      - key: node.kubernetes.io/instance-type
        operator: Exists
```

In this solution, the onus is on the Cloud Provider to implement and pass-down some form of flexibility or minValues down into the scheduler. Cloud Providers could choose to selectively enable or disable this feature by surfacing this API through their NodeClass. 

Enabling this allows specific Cloud Providers to make this change without pushing this new field down into the neutral API; however, this means we need to define a neutral way where a Cloud Provider can define its own flexibility requirements and pass them back to the scheduler. One option here is to allow CloudProviders to layer on their own flexibility requirements for NodePools through the [CloudProvider interface](https://github.com/kubernetes-sigs/karpenter/blob/4e85912c81ed9a51fc8ebd107db8e88e7828fae7/pkg/cloudprovider/types.go). This function in the interface might look like `func GetFlexibility(np *v1beta1.NodePool) scheduling.Flexibility` where `scheduling.Flexibility` is a `map[string]int`.

#### Pros/Cons

1. 👍👍 Isolates the minValues feature to whether a Cloud Provider wants to implement it or not
2. 👍 Enables Cloud Providers to have the concept of “default” minValues that is controlled at runtime, allowing cloud providers to enforce certain minimums from their users for things like spot best practices without having to have the user specify it directly (e.g. “don’t allow a spot instance to launch without at least 10 spot instances in the request”).
3. 👎 Leaks more scheduling behavior into the NodeClass (What if a Cloud Provider doesn’t implement a NodeClass directly? They would have to design one specifically for this feature). As a guiding design principle, we have generally chosen to avoid scheduler-based decision making driven through the NodeClass API
4. 👎 Requires an additional method on the cloud providers which expands our existing interface OR requires piggy-backing off of `GetInstanceTypes()` to surface additional minValues information into the scheduler which may be awkward for cloud providers that don’t care about this requirement

### Solution 3: `flexibility` field in the `spec.requirements` section of the NodeClaim Template (becomes part of the template on the NodePool)

```
apiVersion: karpenter.sh/v1beta1
kind: NodePool
spec:
  template:
    spec:
      requirements:
      - key: karpenter.kwok.sh/instance-family
        operator: In
        values: ["c5", "r3", "m5"]
        flexibility: 2
      - key: node.kubernetes.io/instance-type
        operator: Exists
        flexibility: 10
```

This solution is a natural extension of Solution 1. Rather than simply just enforcing a minimum flexibility on the scheduler, we also enforce a maximum when we go to launch the node. This means that each condition’s values must be **strictly equal** to the flexibility prescribed by the user. This is different from `minValues` where it is only enforcing that the number of options is **strictly greater than or equal**.

As an example, if we had a `node.kubernetes.io/instance-type` field with `flexibility: 2` . If we finished the scheduling loop and had a Preflight NodeClaim with instance type options [“c5.large”, “c5.xlarge”, “c5.4xlarge”], we would truncate our NodeClaims (starting from most expensive to least expensive), attempting to remove instance types until we are at our prescribed flexibility, resulting in [“c5.large”, “c5.xlarge”].

#### Counter Examples

You can quickly see that this breaks-down since it may not be possible to explicitly meet the flexibility that has been prescribed exactly. Imagine conflicting requirements such as:

```
requirements:
- key: karpenter.kwok.sh/instance-generation
  operator: Exists
  flexibility: 1
- key: karpenter.kwok.sh/instance-category
  operator: Exists
  flexibility: 2
```

If our instance type options remaining are : [“c4.large”, “r5.large”], it’s not possible to remove an instance type from the below list to fulfill the flexibility specification. In this case, do we choose to fail the request outright or to continue having gotten as close to the flexibility requirement as possible?

On top of this, the other concern with the flexibility requirement is that (in an effort to fulfill it), you may remove cheaper instance types to drive towards getting the flexibility prescribed by the user. Take, for example, the same requirements listed above but with the instance type options (ordered from cheapest to most expensive) of [“c4.large”, “r5.large”, “c5.xlarge”]. In this case, it **is possible** for us to fulfill the flexibility, but at the expense of throwing out the lowest priced instance type. In either case, this solution makes the problem more complex, while giving us very little value for that additional complexity.

#### Pros/Cons

1. 👍 Allows users to set `flexibility: 1` on instance types to force a lowest price-style allocation strategy for spot capacity
2. 👎👎👎 Enforcing exact flexibility is sometimes not possible and in other cases will cause us to make sub-optimal decisions with respect to price. Explaining this to users would be difficult. This solution is near untenable.

## FAQ

### How do we keep the `minValues` check in the scheduling loop efficient?

A check to `minValues` should be done in the context of the pod `[Add()](https://github.com/kubernetes-sigs/karpenter/blob/990ab8feac42e025eb195afc2f54fd2db059c95c/pkg/controllers/provisioning/scheduling/nodeclaim.go#L65)` simulation and should be performed after the `[filterInstanceTypesByRequirements](https://github.com/kubernetes-sigs/karpenter/blob/990ab8feac42e025eb195afc2f54fd2db059c95c/pkg/controllers/provisioning/scheduling/nodeclaim.go#L104)` call is made. We can validate that we are still fulfilling all of the `minValues` constraints specified by the `requirements` block. To avoid re-iterating through every instance type again and re-counting requirements, we can keep a running count of the number of requirement values for a given key if we care about that key’s flexibility in the Preflight NodeClaim. As we reduce the number of instance types in the Preflight NodeClaim, we can subtract off of these keys and perform checks for `minValues` in constant time.

### How does `minValues` interact with consolidation?

The scheduling simulation should return a result from consolidation that respects `minValues` consistent with the launch behavior. Karpenter will then reduce the set of instance types here to be strictly cheaper than the combined price of all instance types that we plan to replace (this is our `filterByPrice` function).

It’s possible that after Karpenter reduces these instance types down to only cheaper ones, it no longer fulfills the `minValues` criteria of the NodePool requirements. As a result, consolidation will need to re-check whether the `minValues` is met after the instance types are filtered down by price to ensure that we can proceed with the launch. If the `minValues` is not satisfied, we should reject the consolidation decision.

It's important to note here that by setting high `minValues` requirements, you are necessarily constraining the consolidation decisions that Karpenter can make. If Karpenter is forced to maintain a `minValues` of 10 for instance types and you only have 11 instance type options specified, you will only be able to consolidate from the 11th instance type down to one of the bottom 10. At the point that you consolidated down to one of the bottom 10 instance types, you would no longer be able to continue consolidating.

### How does `minValues` interact with spot-to-spot consolidation?

[Spot-to-spot consolidation already requires a spot instance flexibility of 15](https://github.com/kubernetes-sigs/karpenter/blob/main/designs/spot-consolidation.md). If you do not specify any `minValues` across any of your requirements in your NodePool, that 15 instance type minimum will still be required and fulfilled.

If a user has `minValues` set in their NodePool that extend the number of instance types that would be needed for a spot-to-spot consolidation beyond 15, that number of instance types would be maintained. For instance, if a user had the following requirements:

```
- key: node.kubernetes.io/instance-type
  operator: Exists
  minValues: 30
```

In this case, we would maintain 30 instance type values in the launch (rather than truncating down to 15), assuming that the scheduling simulation succeeds. Since 30 > 15 instance types that we require to do spot-to-spot consolidation.

If a user has `minValues` set in their NodePool that require less than 15 instance types, then the 15 instance type minimum will continue to be enforced to enable a spot-to-spot consolidation to occur. For instance, if a user had the following requirements:

```
- key: node.kubernetes.io/instance-type
  operator: Exists
  minValues: 3
```

In this case, we would schedule with at least 3 instance types and then validate that the result allowed for 15 cheaper instance types that we could consolidate to. If there were not 15 cheaper instance types, then we would not perform spot-to-spot consolidation.

### How does `minValues` interact with Drift?

Karpenter currently performs dynamic checking on drift against the NodeClaim labels. Optionally, we could check `minValues` against the `node.kubernetes.io/instance-type` values that are stored in the NodeClaim. Practically, this is probably not necessary as users that want us to respect `minValues` really only care about launch criteria and the nodes on the cluster have already been launched.

**This is also a fast-follow decision that we could implement if we got an ask for supporting drift with this new `minValues` field.**

### What if my NodePool and Pod requirements can’t fulfill my request for flexibility?

If Karpenter has already packed too many pods on a Preflight NodeClaim in a way that would cause the instance type options to not fulfill the `minValues` requirement, Karpenter will create another Preflight NodeClaim in the simulation rather than continuing to pack pods on the previous NodeClaim. If a single pod cannot schedule to a Karpenter NodePool with `minValues` configured without breaking the constraint, Karpenter will fallback to the next weighted NodePool on the cluster, which may include a NodePool that doesn’t directly support `minValues`.
