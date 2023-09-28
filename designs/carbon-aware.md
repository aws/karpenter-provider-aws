# Carbon Aware Karpenter: Optimizing Kubernetes Cluster Autoscaling for Environmental Sustainability
*Author: [@JacobValdemar](https://github.com/JacobValdemar)*

## Context & Problem
There is a growing concern about environmental sustainability within the context of Kubernetes cluster autoscaling. In multiple comments on [the proposal for moving Karpenter to CNCF](https://github.com/kubernetes/org/issues/4258), the move is backed because of opportunities within environmental sustainability.

I'm currently working on my master's thesis in Computer Engineering (M.Sc.Eng) at Aarhus University located in Denmark. The objective of the thesis is to enable Karpenter to minimize carbon emissions from Kubernetes clusters that run on cloud infrastructure (focus is AWS).

RFC: https://github.com/aws/karpenter/issues/4630

## Fundamentals of Green Software
I'll try to keep it simple. The reader should be familiar with the following.

A cluster's emissions is made of two elements: embodied emissions and operational emissions. To get the total emissions, one can add them togeather.

- **Embodied carbon emissions**: Manufacturing emissions (CO₂e) amortized over instance lifetime (usually 4 years) divided by how long we use the instance
- **Operational carbon emissions**: Carbon emitted by electricity grid to produce electricity for the instance in the region where it is used, multiplied by PUE

There is a lot more to Green Software. If you want to learn more, I recommend you to visit [Green Software Practitioner](https://learn.greensoftware.foundation/) (a Green Software Foundation project - an affiliate of the Linux Foundation).

## Solution

### Feature Gate
The feature is proposed to be controlled using a [feature gate](https://karpenter.sh/docs/concepts/settings/#feature-gates).

| **Feature**  | **Default** |         **Config Key**          | **Stage** |    **Since**    | **Until** |
| :----------: | :---------: | :-----------------------------: | :-------: | :-------------: | :-------: |
| CarbonAware |    false    | featureGates.carbonAwareEnabled |   Alpha   | v0.31.0/v0.32.0 |           |

### Carbon emissions data source
Currently the best option seems to be to create estimates based on the methodology used in [Boaviztapi](https://github.com/Boavizta/boaviztapi). [Data demo](https://datavizta.boavizta.org/cloudimpact).

I assume that we only want to use "static" data so we don't have to go out and make requests over the Internet to get real time data, since that would be expensive for performance.

#### Limitations
For both options above, there is a discrepancy between the available instances known to Karpenter and instances know to the carbon emissions data source. This means that as it is right now, it is not possible to get carbon emissions data for all instances types. This is mostly the case for new instance types such as m7g. Unfurtunately this seems to extend to around 300 out of 700 instance types. See full comparison in [this Gist](https://gist.github.com/JacobValdemar/e1342013c0f5c980126f6a1feb66b4a1).

I will attempt to eleminate this discrepancy, but it might not be possible. It will probably not always be possible to have an updated list of estimated carbon emissions for all instances as AWS continue to release new instance types. We should consider what to do with instance types that we do not have carbon emission estimates for.

Approaches to handle this:
1. Estimate extremely high emissions to effectively filter out unknown instance types
2. Estimate zero emissions

I recommend option 1, as option 2 could potentially make the cluster even worse, environmentally.

### Launch strategy
To enable emission based priotization, the launch strategy should be changed from `lowest-price` to `prioritized`.

### Changes to consolidation (karpenter-core)
Single Machine Consolidation (`singlemachineconsolidation.go`) and Multi Machine Consolidation (`multimachineconsolidation.go`) as well as `consolidation.go` is currently consolidating nodes to reduce costs. We want to change this when Carbon Aware is enabled. They should consolidate to minimize carbon emissions. 

### Changes to Provisioning
Currently, provisioning (roughly) filter instances based on requirements, sort instances by price, and launch the cheapest instance. We want to change this when Carbon Aware is enabled. It should sort instances by carbon emissions and launch the instance which has the lowest Global Warming Potential[^1].

### Option 1: Use Carbon Aware provisioning and concolidation methods

#### Consolidation
Create two new consolidation methods `carbonawaresinglemachineconsolidation.go` and `carbonawaremultimachineconsolidation.go` that will be used when Carbon Aware is enabled.

<details>

<summary>Change to `karpenter-core/pkg/controllers/deprovisioning/controller.go`</summary>

```diff
-func NewController(clk clock.Clock, kubeClient client.Client, provisioner *provisioning.Provisioner,
-	cp cloudprovider.CloudProvider, recorder events.Recorder, cluster *state.Cluster) *Controller {
+func NewController(ctx context.Context, clk clock.Clock, kubeClient client.Client, provisioner *provisioning.Provisioner,
+	cp cloudprovider.CloudProvider, recorder events.Recorder, cluster *state.Cluster) *Controller {

+	if settings.FromContext(ctx).CarbonAwareEnabled {
+		return &Controller{
+			clock:         clk,
+			kubeClient:    kubeClient,
+			cluster:       cluster,
+			provisioner:   provisioner,
+			recorder:      recorder,
+			cloudProvider: cp,
+			lastRun:       map[string]time.Time{},
+			deprovisioners: []Deprovisioner{
+				NewExpiration(clk, kubeClient, cluster, provisioner, recorder),
+				NewDrift(kubeClient, cluster, provisioner, recorder),
+				NewEmptiness(clk),
+				NewEmptyMachineConsolidation(clk, cluster, kubeClient, provisioner, cp, recorder),
+				NewCarbonAwareMultiMachineConsolidation(clk, cluster, kubeClient, provisioner, cp, recorder),
+				NewCarbonAwareSingleMachineConsolidation(clk, cluster, kubeClient, provisioner, cp, recorder),
+			},
+		}
+	}

	return &Controller{
		clock:         clk,
		kubeClient:    kubeClient,
		cluster:       cluster,
		provisioner:   provisioner,
		recorder:      recorder,
		cloudProvider: cp,
		lastRun:       map[string]time.Time{},
		deprovisioners: []Deprovisioner{
			NewExpiration(clk, kubeClient, cluster, provisioner, recorder),
			NewDrift(kubeClient, cluster, provisioner, recorder),
			NewEmptiness(clk),
			NewEmptyMachineConsolidation(clk, cluster, kubeClient, provisioner, cp, recorder),
			NewMultiMachineConsolidation(clk, cluster, kubeClient, provisioner, cp, recorder),
			NewSingleMachineConsolidation(clk, cluster, kubeClient, provisioner, cp, recorder),
		},
	}
}
```
</details>

#### Provisioning
In karpenter-core, create a new method `types.go/OrderByCarbonEmissions` and use that in `nodeclaimtemplate.go/ToMachine` and `nodeclaimtemplate.go/ToNodeClaim` instead of `types.go/OrderByPriceEmissions` if Carbon Aware is enabled.

In karpenter, create a new method `CarbonAwareCreate` in `pkg/providers/instance/instance.go` that is used in `pkg/cloudprovider/cloudprovider.go/Create` instead of `pkg/providers/instance/instance.go/Create` when Carbon Aware is enabled.

#### Considerations
1. 👍 Current consolidation methods are unaffected
1. 👍 Following the principle *Push back on requirements that introduces concepts for all users to solve problems for a few*
1. 👎 There might be copy-paste of code from the original consolidation methods to the carbon aware consolidators

### Option 2: Use Carbon Aware filtering/sorting methods

#### Consolidation
Create carbon aware implementations of low-level functions like `filterByPrice`, `filterOutSameType`, `getCandidatePrices`, etc. that is used when Carbon Aware is enabled. Usage of aforementioned functions might assume that it is price that they are getting, but in reality it is data about carbon emissions.

#### Provisioning
Use same changes to provisioning as in [option 1](#option-1-use-carbon-aware-provisioning-and-concolidation-methods).

#### Considerations
1. 👍 Less code copy-paste
1. 👍 Improvements to original consolidation methods also improve the Carbon Aware feature
1. 👎 Has a risk of breaking undocumented invariants
1. 👎 Adds complexity to the original consolidation methods

### Option 3: Put a price on carbon emissions and account for it
Set a price per tonne (or kg) of CO₂e.

Maybe have a config option for this, defaulting to $0. A good starting value for taking environmental impact into account is $0.25 / kg. (source missing)

Adjust the price algorithm to account for the pollution cost supplement.

Additionally, if actual cost is no issue, we could weight them:
- 100% weighting to the inferred pollution cost
- 0% weighting to the billed cost from AWS

This applies to both provisioning and consolidation.

#### Considerations
1. 👍 Change is constrained to the pricing domain, so most of Karpenter's logic remains unaffected.
1. 👍👍 Makes it possible to combine pricing and emission factors for a balanced solution that might have a good appeal to standard users. This could make the feature appealing for general availability using a carbon price recommended by a trusted organization like the United Nations.
1. 👎 The operational carbon emission estimate is of highest quality if we use the instance utilization in the calculation. Depending on where and how the Carbon Aware feature is added, it might be hard to access to current and simulated instance utilization information. If the emission estimate and thus the price depends on utilization, the price will be different for different combinations of instances. This might break an undocumented invariant or be incompatible with the current architecture, requiring significant redesign if we want to use actual instance utilization in the calculation. For example, `m6g.4xlarge` could have a GWP of 1887 kgCO₂e or 2347 kgCO₂e at respectivly 10% or 100% load[^2]. One could of course just make assumptions, but that wont give the most accurate estimation. This is a trade off.
1. 👎 Adds complexity to the *price* concept. Price is not just price, but rather becomes an optimization function.
1. 👎 Without any adaptation, the `karpenter_cloudprovider_instance_type_price_estimate` metric will represent more than just price when Carbon Aware is enabled.

### Option 4: Enable custom instance price overrides
A different approach could be to enable users to configure custom instance price overrides, e.g. in a ConfigMap. A configuration using emission factors (varying with region and instance type) masked as prices can be pre-generated. Users then copy-paste a Carbon Aware `priceOverride` into their environment.

```yaml
priceOverrides:
  - name: m5.large
    zone: us-west-2a
    capacity-type: spot
    price: .001
  - name: m5.xlarge
    zone: us-west-2a
    capacity-type: spot
    price: .002
```

#### Considerations
1. 👍 Can be used for other purposes
1. 👎 Carbon emission price can not be combined with actual price
1. 👎 Operational carbon emissions can not use dynamic carbon intensity (possible future feature)
1. 👎 Operational carbon emissions can not use current or simulated node utilization

[^1]: The potential impact of greenhouse gases on global warming. Measured in terms of CO₂e.
[^2]: https://dataviz.boavizta.org/cloudimpact
