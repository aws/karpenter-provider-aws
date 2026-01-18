# Disruption Controls By Reason
## User Stories
1. Users need the capability to schedule upgrades only during business hours or within more restricted time windows. Additionally, they require a system that doesn't compromise the cost savings from consolidation when upgrades are blocked due to drift.
2. Users want to minimize workload disruptions during business hours but still want to be able to delete empty nodes throughout the day.  That is, empty can run all day, while limiting cost savings and upgrades due to drift to non-business hours only.

See Less Made Up Scenarios here: 
- https://github.com/kubernetes-sigs/karpenter/issues/924 
- https://github.com/kubernetes-sigs/karpenter/issues/753#issuecomment-1790110838
- https://github.com/kubernetes-sigs/karpenter/issues/672
- https://github.com/kubernetes-sigs/karpenter/issues/1179 

See further breakdown of some additional scenarios towards the bottom of the document 

## Clarifying the requirements and behavior 
**Reason and Budget Definition:** Users should be able to define an reason and a corresponding budget(s).
**Supported Reasons:** All disruption Reasons affected by the current Budgets implementation (Underutilized, Empty, Drifted) should be supported. 
**Default Behavior for Unspecified Reasons:** Budgets should continue to support a default behavior for all disruption reasons. 

# API Design
### Approach A: List Approach With Multiple Reasons per Budget - Recommended
This approach allows specifying multiple disruption methods within a single budget entry. It is proposed to add a field Reasons to the budgets, which can include a list of reasons this budget applies to.
#### Proposed Spec
Add a simple field "reasons" is proposed to be added to the budgets. 
```go
// Budget defines when Karpenter will restrict the
// number of Node Claims that can be terminating simultaneously.
type Budget struct {
      // Reasons is a list of disruption reasons. If Reasons is not set, this budget applies to all methods.
      // If a reason is set, it will only apply to that method. If multiple reasons are specified,
      // this budget will apply to all of them. If a reason is unspecified we will take the min value of this budget and the rest of the active budgets.
      // if an unspecified reason exists we will also override all other reasons with its value if they are smaller than the unspecified reason.
      // allowed reasons are "Underutilized", "Empty", "Drifted"
      // +kubebuilder:validation:UniqueItems
      // +kubebuilder:validation:MaxItems=4
      // +kubebuilder:validation:Enum:={"Underutilized","Empty","Drifted"}
      // +optional
      Reasons []string `json:"reason,omitempty" hash:"ignore"`
      // Nodes dictates the maximum number of NodeClaims owned by this NodePool
      // that can be terminating at once. This is calculated by counting nodes that
      // have a deletion timestamp set, or are actively being deleted by Karpenter.
      // This field is required when specifying a budget.
      // This cannot be of type intstr.IntOrString since kubebuilder doesn't support pattern
      // checking for int nodes for IntOrString nodes.
      // Ref: https://github.com/kubernetes-sigs/controller-tools/blob/55efe4be40394a288216dab63156b0a64fb82929/pkg/crd/markers/validation.go#L379-L388
      // +kubebuilder:validation:Pattern:="^((100|[0-9]{1,2})%|[0-9]+)$"
      // +kubebuilder:default:="10%"
      Nodes string `json:"nodes" hash:"ignore"`
      // Schedule specifies when a budget begins being active, following
      // the upstream cronjob syntax. If omitted, the budget is always active. 
      // Timezones are not supported. This field is required if Duration is set. 
      // +kubebuilder:validation:Pattern:=`^(@(annually|yearly|monthly|weekly|daily|midnight|hourly))|((.+)\s(.+)\s(.+)\s(.+)\s(.+))$` 
      // +optional Schedule *string `json:"schedule,omitempty" hash:"ignore"` 
      // Duration determines how long a Budget is active since each Schedule hit. 
      // Only minutes and hours are accepted, as cron does not work in seconds.
      // If omitted, the budget is always active.
      // This is required if Schedule is set.
      // This regex has an optional 0s at the end since the duration.String() always adds
      // a 0s at the end.
      // +kubebuilder:validation:Pattern=`^([0-9]+(m|h)+(0s)?)$`
      // +kubebuilder:validation:Type="string"
      // +optional
      Duration *metav1.Duration `json:"duration,omitempty" hash:"ignore"`
}

```
#### Example
```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec: # This is not a complete NodePool Spec.
  disruption:
    budgets:
    - schedule: "* * * * *"
      reasons: [Drifted, Underutilized]
      nodes: 10
    # For all other reasons, only allow 5 nodes to be disrupted at a time
    - nodes: 5
      schedule: "* * * * *"

```

In the original proposed spec, karpenter allows the user to specify up to [50 budgets](https://github.com/kubernetes-sigs/karpenter/blob/main/pkg/apis/v1beta1/nodepool.go#L96)
If there are multiple active budgets, karpenter takes the most restrictive budget. This same principle will be applied to the disruption budgets in this approach. The only difference in behavior is that each window will apply to list of reasons that are specified rather than just all disruption methods. 
### Pros + Cons 
* 👍👍 Flexibility in Budget Allocation: Allows more flexibility in allocating budgets across multiple disruption reasons.
* 👍👍 Reduced Configuration Complexity: Simplifies the configuration process, especially for similar settings across multiple reasons.
* 👎 Potential for API Complexity: There might be confusion over whether the node count is shared between actions or if each action gets the node count individually. 

Note some pros and cons between A + B can be shared, and are in a list next to the pros + cons for Single Reason Approach 

#### Approach B: Single Reason Approach With One Reason Per Budget
### Proposed Spec
In this approach, each budget entry specifies a single reason for disruption.

```go
// Budget defines when Karpenter will restrict the
// number of Node Claims that can be terminating simultaneously.
type Budget struct {
      // +optional
      // +kubebuilder:validation:Enum:={"Underutilized","Empty","Drifted"}
      Reason string `json:"reason,omitempty" hash:"ignore"`
      // Nodes dictates the maximum number of NodeClaims owned by this NodePool
      // that can be terminating at once. This is calculated by counting nodes that
      // have a deletion timestamp set, or are actively being deleted by Karpenter.
      // This field is required when specifying a budget.
      // This cannot be of type intstr.IntOrString since kubebuilder doesn't support pattern
      // checking for int nodes for IntOrString nodes.
      // Ref: https://github.com/kubernetes-sigs/controller-tools/blob/55efe4be40394a288216dab63156b0a64fb82929/pkg/crd/markers/validation.go#L379-L388
      // +kubebuilder:validation:Pattern:="^((100|[0-9]{1,2})%|[0-9]+)$"
      // +kubebuilder:default:="10%"
      Nodes string `json:"nodes" hash:"ignore"`
      // Schedule specifies when a budget begins being active, following
      // the upstream cronjob syntax. If omitted, the budget is always active.
      // Timezones are not supported.
      // This field is required if Duration is set.
      // +kubebuilder:validation:Pattern:=`^(@(annually|yearly|monthly|weekly|daily|midnight|hourly))|((.+)\s(.+)\s(.+)\s(.+)\s(.+))$`
      // +optional
      Schedule *string `json:"schedule,omitempty" hash:"ignore"`
      // Duration determines how long a Budget is active since each Schedule hit.
      // Only minutes and hours are accepted, as cron does not work in seconds.
      // If omitted, the budget is always active.
      // This is required if Schedule is set.
      // This regex has an optional 0s at the end since the duration.String() always adds
      // a 0s at the end.
      // +kubebuilder:validation:Pattern=`^([0-9]+(m|h)+(0s)?)$`
      // +kubebuilder:validation:Type="string"
      // +optional
      Duration *metav1.Duration `json:"duration,omitempty" hash:"ignore"`
}

```
##### Example
```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: default
spec: # This is not a complete NodePool Spec.
  disruption:
    budgets:
    - schedule: "* * * * *"
      reason: drifted 
      nodes: 10
    # For all other reasons , only allow 5 nodes to be disrupted at a time
    - nodes: 5
      schedule: "* * * * *"

```


#### Pros and Cons
* 👍 Simplicity and Clarity: Offers a straightforward and clear mapping of budget to disruption reason.
* 👎 Increased Configuration Overhead: Requires duplicating settings for multiple reasons, increasing setup complexity.
* 👎 Less Flexibility: Lacks the flexibility to share a budget across multiple reasons.

#### Pros and Cons for List and Per Reason Definitions
Some of the Pros and Cons are shared by both list and single reason, as they have the same advantages and disadvantages in comparison to Per Reason Controls 
* 👍👍 Extends Existing API:  No Breaking API Changes, completely backwards compatible
* 👍 No Nesting Required: Leaves budgets at the top level of the api.
* 👎 Limited Generalization of Reason Controls: With reason being clearly tied to budgets, and other api logic being driven by disruption reason, we lose the chance to generalize per Reason controls. If we ever decide we need a place per action,  there will be some duplication for reason. 

### Approach C: Defining Per Reason Controls  
Ideally, we could move all generic controls that easily map into other reasons into one set of reason controls, this applies to budgets and other various disruption controls that could be more generic. 
#### Example 

```yaml 
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: example-nodepool
spec:
  disruption:
    defaults:
      budgets: 
        - nodes: 10% 
          schedule: "0 0 1 * *"
          duration: 1h 
    consolidation:
      consolidationPolicy: WhenUnderutilized
      disruptAfter: "30m"
    drift:
      budgets:
        - nodes: "20%"
          schedule: "0 0 1 * *"
          duration: "1h"
        - nodes: "10%"
          schedule: "0 0 * * 0"
          duration: "2h"
        - nodes: "50%" 
          schedule: "@monthly"
```
#### Considerations 
Some of the API choices for a given reason seem to follow a similar pattern. These include ConsolidateAfter, ExpireAfter. Moreover, when discussing disruption budgets, we talk about adding behavior for each reason. It appears there is a need for disruption controls within the budgets for each reason, not just overall.

This approach aligns well with controls that apply to all existing reasons. The proposal presented here is similar to the one mentioned above in relation to the reasons we allow to be defined (Underutilized, Drifted, Empty).

This proposal is currently scoped for disruptionBudgets by reason. However, we should also consider incorporating other generic disruption controls into the PerReasonControls, even if we do not implement them immediately. Moving ConsolidateAfter and ExpireAfter into the per-reason controls is a significant migration that requires careful planning and its own dedicated design. This proposal simply demonstrates a potential model that highlights the benefits of defining controls at a per-reason level of granularity.

### Pros + Cons 
* 👍 Granular Control for Each Reason: This model aligns well with the need for specific controls for each disruption reason, providing a natural specification for further granularity. 
* 👍 Foundation for Future Extensions: Offers a framework for incorporating other generic disruption controls into the per-reason controls, extending beyond budgets.
* 👍👍 Customization for Specific Reasons: Allows for specific validations and extensions for individual reasons, offering tailored control.
* 👎👎👎 Breaking API Changes:Implementing this approach would necessitate significant changes to the current API, especially for budgets. If we decide to model DisruptAfter in a similar way we also would have to break those apis. It might make sense to break the budgets now before they have garnered large adoption as it becomes harder to make the change as time goes on.
* 👎 Complexity in Default Reason Handling: The model does not facilitate easy defaulting for unspecified ("default") disruption reasons.
* 👎 Increased Budget Complexity: The use of budgets becomes more complex as they are now nested within another field, adding a layer of intricacy to their application.

### API Design Conclusion: Extend Disruption Budgets To Have A List Of Reasons 
After evaluating different approaches to extend the Karpenter API for specifying disruption reasons, the preferred design is the List Approach in Approach A. This approach offers flexibility in managing multiple disruption reasons under a single budget and reduces configuration complexity. It extends the existing API without introducing breaking changes and simplifies management for scenarios where multiple disruption reasons share similar constraints.

While the idea of per-reason controls (Approach C) provides granular control and a foundation for future extensions, it involves significant API changes and increased complexity, making it less favorable at this stage. However, this approach remains a viable option for future considerations, especially if there is a need for more tailored control over each disruption reason.

### Counting Disruptions Against a Budget 
The calculation of allowed disruptions in a system with multiple disruption reasons has become more intricate following the introduction of Disruption Budget Reasons. Previously, the formula was straightforward:

AllowedDisruptions = minNodeCount - unhealthyNodes - totalDisruptingNodes.

When calculating by reason, three potential equations emerge:
1.(Recommended) minimumDisruptionAllowed = min(minNodeCount[reason], minNodeCount[default]) - unhealthyNodes - totalDisruptingNodes
2. BucketedDisruptionByReason = minNodeCount[reason] - unhealthyNodes - totalDisruptingNodes[reason]
3. AllowedDisruptionByReason = minNodeCount[reason] - unhealthyNodes - totalDisruptingNodes


#### Recommendation: minimumDisruptionAllowed equation(AKA Equation 1)
Equation 1 is recommended as its the least intrusive change to the disruption equation, and avoids edge cases we see with the bucketed approach. Equation 2 doesn't follow the min budget principle that we have established in the original disruption controls api.
For these reasons we can go ahead with equation 1. We will walk through the customer scenarios using nodepools defined with the disruption behavior being driven from equation three below to ensure our API can perform all of the customer asks we have received so far here. 

### User Stories
We will go through scenarios that users are expecting to face to drive home if the api with equation 1 can satisfy the users requirements for the end users of these features
- Block Drift, but allow Expiration
- Fully Block Drift && Expiration, but allow consolidation to occur 
- Block Drift && Expiration, but allow emptiness 
- Limit Drift && Expiration while allowing aggressive but not unbounded consolidation

Note that these samples are written with the intention to think through our design, and more formal documentation will be prepared for common customer scenarios.

#### Block Drift and allow Expiration 
Customers who roll out aggressive node image upgrades don't want to roll the nodes due to drift, but want to use node expiration to force users onto compliant node images eventually. The scenario here is an ML Training workload. Ideally we want to let it bake as long as possible, but not at the cost of compromising the security of the cluster. 

```yaml 
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: worker-pool 
spec: # This is not a complete NodePool Spec.
  disruption:
    consolidationPolicy: WhenEmpty 
    expireAfter: 7d 
    budgets:
    - nodes: 0
      reasons: [drifted] 
    - nodes: "100%" // All Disruption Other than drift(Expiration and Empty Nodes) are allowed
```

We could also allow a semantic like this
```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: worker-pool 
spec: # This is not a complete NodePool Spec.
  disruption:
    consolidationPolicy: WhenEmpty 
    expireAfter: 7d 
    budgets:
    - nodes: "100%" # Always allow Expiration and Emptiness to disrupt 
      reasons: [expired, empty]
    - nodes: 0
```
So in this case, we would take the minimum node count, and it would block the 100% expiration budget of expired and Empty. 
But this semantic would not work for equation 1. Equation 1 will take the minimum between minNodeCount[reason] and the minNodeCount[default/unspecifed].  

#### Fully Blocking Drift && Expiration while allowing consolidation
```yaml 
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: worker-pool 
spec: # This is not a complete NodePool Spec.
  disruption:
    consolidationPolicy: WhenUnderutilized 
    expireAfter: Never 
    budgets:
    - nodes: 0
      reasons: [drifted, expired] 
    - nodes: "100%" // All Disruption Other than drift and expiration are allowed
```

#### Fully Blocking Drift and Expiration while allowing emptiness 
```yaml 
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: worker-pool 
spec: # This is not a complete NodePool Spec.
  disruption:
    consolidationPolicy: WhenEmpty
    expireAfter: Never 
    budgets:
    - nodes: 0
      reasons: [drifted] 
    - nodes: "100%" // All Disruption Other than drift(Expiration and Empty Nodes) are allowed
```

#### Limiting Drift & Expiration while allowing aggressive but not unbounded consolidation
Drift and Expiration may not be desired reasons to remove many nodes in one interval, but Underutilization and Emptiness may be desired. We may want to still consolidate at a healthy threshold but not allow drift or expiration to trigger a [mass exodus of node removals](https://github.com/kubernetes-sigs/karpenter/issues/1179#issuecomment-2049160258).

This avoids the situation where we have 100s of drift or expirations all lining up at once and mass rotating all active nodes at once.
```yaml
apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: worker-pool 
spec: # This is not a complete NodePool Spec.
  disruption:
    consolidationPolicy: WhenUnderutilized 
    expireAfter: 1h 
    budgets:
    - nodes: 1
      reasons: [drifted, expired] # Only allow Drift and Expiration to occur one at a time 
    - nodes: "33%" # All Consolidation actions taken either through replace or delete should be allowed to disrupt at least a third of the cluster at once 
```



### Q: How should we handle an unspecified reason when others are specified? 
```yaml
budgets: 
  - nodes: 10
    reasons: [Drifted, Underutilized]
    schedule: "* * * * *"
  - nodes: 5 
    reasons: [Empty] 
    schedule: "* * * * *" 
```
In the case of a budget like above, default is undefined. Should karpenter assume the user doesn't want to disrupt any other reasons? Or should we assume that if a default is unspecified, they want us to disrupt anyway?  
The intuitive options if there is no active default budget is to allow disruption of either 0 or total number of nodes(meaning unbounded disruption).
Lets choose total number of nodes, since this allows the user to also specify periods where no nodes are to be disrupted of a particular type of disruption, and makes more sense with the existing karpenter behavior today.
