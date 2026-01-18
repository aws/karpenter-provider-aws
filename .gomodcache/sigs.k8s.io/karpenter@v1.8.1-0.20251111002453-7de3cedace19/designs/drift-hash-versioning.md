# Karpenter Drift Hash Versioning with CRD

## Summary of Static Drift Hashing Mechanism

Drift utilizes a hashing mechanism to identify when a `NodeClaim` has been drifted from its owning `NodePool`. Karpenter will hash fields that are within the `NodePool.Spec.Template`, excluding [special cases of drift](https://karpenter.sh/docs/concepts/disruption/#special-cases-on-drift) or [behavior fields](https://karpenter.sh/docs/concepts/disruption/#behavioral-fields). The `NodePool` will be annotated with the hash. When a `NodeClaim` is created from this NodePool, the same `NodePool` hash gets propagated down to the `NodeClaim`. More details can be found in the [drift documentation](https://karpenter.sh/docs/concepts/disruption/#drift). 

When a `NodePool` changes, the hash value of the `NodePool` also changes. The `NodeClaim` disruption controller will periodically check if the `NodePool` hash matches the `NodeClaim`'s hash to determine when a `NodeClaim` is drifted.

## Background

Karpenter's static drift mechanism works well when adding new fields to the CRD that don't have defaults, but breaks down if there is a breaking change to the `Nodepool` hashing mechanism. If a cloud provider makes one of these changes to their CRDs, the hash annotation value would change, causing all nodes to roll when the CRD is upgraded. This causes problems for cloud providers when maintainers need to make these kinds of changes (due to requirements changing or bugs) but don't want to impact users by rolling nodes. There are currently two issues that would force our drift hashing value to change:

1. Setting cloud provider defaults for the: `nodeClassRef.apiVersion` and `nodeClassRef.kind` fields [#909](https://github.com/kubernetes-sigs/karpenter/issues/909)
2. Fixing `volumeSize` to be correctly considered for hashing [#5447](https://github.com/aws/karpenter-provider-aws/issues/5447)

More generally, these are the types of breaking changes to the hashing function that should not induce drift:

1. A `NodePool` adds default values to an existing field
2. A `NodePool` field is added with a default
3. A `NodePool` field is removed

## Solutions

### Option 1: Add a hash version annotation and fallback to rehashing on the NodeClaim 

As the Karpenter CRDs change, Karpenter’s underlying hashing function may also change. To handle the upgrade path, Karpenter will add a `nodepool-hash-version` into the karpenter controller. The `nodepool-hash-version` will be bumped when a breaking change is made to the hash function. The version bump will be done by the maintainers. The `nodepool-hash-version` will look as such:

```
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
  annotations: 
    karpenter.sh/nodepool-hash: abcdef
    karpenter.sh/nodepool-hash-version: v1
    ...
---
apiVersion: karpenter.sh/v1beta1
kind: NodeClaim 
metadata:
  name: default
  annotations: 
    karpenter.sh/nodepool-hash: abcdef
    karpenter.sh/nodepool-hash-version: v1
    ...
```

When Karpenter is spun up, the controller will check the `nodepool-hash-version`, and if the version on the NodePool doesn't match the version in the controller, re-calculate the `NodePool` hash and propagate the annotation down to the `NodeClaim`. Karpenter will assert that this `nodepool-hash-version` annotation is equivalent on both the `NodePool` and `NodeClaim` before doing a drift evaluation. 

As a result, two annotations will be used analyze and orchestrate drift: 

* nodepool-hash: (hash of the nodeclaim template on the nodepool)
* nodepool-hash-version: (the NodePool hashing version defined by Karpenter)

Option 1 has two cases for drift:

1. When the CRD `nodepool-hash-version` matches between the `NodePool` and `NodeClaim`:
    1. Drift will work as normal and the `NodePool` hash will be compared to evaluate for drift 
2. When the CRD `nodepool-hash-version` does not match between the `NodePool` and `NodeClaim`:
    1. Karpenter would recalculate the hash of the `NodePool` and back-propagate the updated value for the `karpenter.sh/nodepool-hash` annotation to the NodeClaims. Any `NodeClaims` that are already considered drifted will remain drifted if the `karpenter.sh/nodepool-hash-version` doesn't match

Pros/Cons

- 👍👍 Simple implementation effort 
- 👍 Covers general cases of updates to the `NodePool` CRD or the hashing mechanism
- 👎 Nodes that were considered drifted before the Karpenter upgrade are not able to be un-drifted after a Karpenter upgrade 
- 👎 Users updating the NodePool while the Karpenter controller is down, during a hash version change, will find NodeClaims not to have nodes drift.

### Option 2: Add a hash version annotation and fallback to individual field evaluation for drift

Similar to Option 1, there are two cases for drift when using `nodepool-hash-version`:

1. When the `nodepool-hash-version` matches between the `NodePool` and `NodeClaim`:
    1. Drift will work as normal and the `nodepool-hash` will be compared to evaluate for drift 
2. When the `nodepool-hash-version` does not match between the `NodePool` and `NodeClaim`:
    1. Karpenter will individually check each field for drift. *This would be an adaptation of the (Deployment Controller)[https://github.com/kubernetes/kubernetes/blob/83e6636096548f4b49a94ece6bc4a26fd67e4449/pkg/controller/deployment/util/deployment_util.go#L603] for podTemplateSpec relationship to pods.*
    2. Karpenter will need to populate the `NodeClaim` with values that were used in launching the instances. Drift evaluation will need a point of reference on manual checking.
    3. The cloudprovider will need to add the `NodeClass` details on the `NodeClaim` for individual field evaluation at NodeClaim creation. Cloudprovider will need to add a JSON annotation of the NodeClass on to the `NodeClaim`.  

Pros/Cons

- 👍 Cover general cases of updates to the `NodePool` CRD or the hashing mechanism 
- 👍 Remove the race condition of having a new hash being propagating down to the `NodeClaims`
- 👎 Cloud-provider would need to propagate on to the NodeClaim to adopt this method for static drift checking 
- 👎👎 A high maintenance effort required in supporting individual field evaluation 

### Option 3: Add a hash version annotation, bumping the NodePool API Version to `v1beta2`, utilizing conversion webhooks to update the `nodepool-hash`

The Karpenter APIVersion of the CRDs will be bumped when there is a breaking change to the hashing mechanism described above. As part of retrieving the `NodePool` from the API Server, a conversion webhook would re-calculate the hash, allowing Karpenter will propagate the new hash to the `NodeClaims`.

As a result, we will be coupling the `karpenter.sh/nodepool-hash` hashing mechanism to the apiVersion and we will treat any breaking change to the hash versioning as a breaking change to the CRD. Similar to Option 1, once the conversion webhooks will return an updated `nodepool-hash-version`. This will trigger a hash propagation for `karpenter.sh/nodepool-hash` to all the `NodeClaims`.

Pros/Cons 
- 👍 Covers general cases of updates to the `NodePool` CRD or the hashing mechanism 
- 👎 Maintain conversion webhooks 
- 👎👎 In the case of adding a new field with defaults, the API version would be bumped for non-breaking changes. Users running Karpenter would have to bump their manifests to use the new apiVersion to work more frequently

### Option 4: Switch to individual field evaluation for drift

Another option Karpenter to switch completely to checking each field on the `NodePool` against each field on the `NodeClaim`. This option will assume that all fields that used to validate drift will be found on the `NodeClaim`.

Pros/Cons 

- 👍 Consistent drift behavior for upgrading the Karpenter CRDs and standard drift evaluation   
- 👎 Cloud-provider would need to propagate on to the NodeClaim to adopt this method for static drift checking 
- 👎👎 A high maintenance effort required in supporting individual field evaluation 

## Recommendation

The main goal we are trying to achieve for drift hash versioning is stability on node churn for clusters during a Karpenter upgrade with updates to the `NodePool` CRDs. All four options will prevent Karpenter from automatically drifting nodes during a Karpenter upgrade. Option 1 is recommended as it will give the maintainers more control over when the the drift hash will need to be updated. The advantage of Option 1 is that it would give Karpenter the ability to correctly identify drifted nodes when `NodePool` are changed during a Karpenter upgrade. However, customers that update their `NodePools` resources could run into a race condition, where Karpenter would consider `NodeClaims` not drifted, if they update their `NodePools` while Karpenter is propagating new hashes to `NodeClaims`. The team will mitigate this issue by warning and documenting this side effect to users.