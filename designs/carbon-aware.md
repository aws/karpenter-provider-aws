# Carbon Aware Karpenter: Optimizing Kubernetes Cluster Autoscaling for Environmental Sustainability
*Author: [@JacobValdemar](https://github.com/JacobValdemar)*

## Context & Problem
There is a growing concern about the environmental impact of Kubernetes clusters. Karpenter's opportunities within environmental sustainability is referenced in multiple comments that back [`karpenter-core`'s move to CNCF](https://github.com/kubernetes/org/issues/4258).

I am currently working on my master's thesis in Computer Engineering (Master of Science in Engineering) at Aarhus University located in Denmark. The objective of the thesis is to enable Karpenter to minimize carbon emissions from Kubernetes clusters that run on cloud infrastructure (scoped to AWS).

RFC: https://github.com/aws/karpenter/issues/4630

## Fundamentals of Green Software
I will try to keep it simple. The reader should be familiar with the following.

A cluster's emissions is made of two elements: embodied emissions and operational emissions. To get the total emissions, one can add them togeather.

- **Embodied carbon emissions**: Manufacturing emissions (CO₂e) amortized over instance lifetime (usually 4 years) divided by how long we use the instance
- **Operational carbon emissions**: Carbon emitted by electricity grid to produce electricity for the instance in the region where it is used, multiplied by PUE

An instance is a share of a physical machine, so when we refer to "instance lifetime" we mean the lifetime of the physical machine that the instance partially constitutes.

There is a lot more to Green Software. If you want to learn more, I recommend you to visit [Green Software Practitioner](https://learn.greensoftware.foundation/) (a Green Software Foundation project - an affiliate of the Linux Foundation).

## Solution

### Feature Gate
The feature is proposed to be controlled using a [feature gate](https://karpenter.sh/docs/concepts/settings/#feature-gates).

| **Feature** | **Default** |         **Config Key**          | **Stage** |    **Since**    | **Until** |
| :---------: | :---------: | :-----------------------------: | :-------: | :-------------: | :-------: |
| CarbonAware |    false    | featureGates.carbonAwareEnabled |   Alpha   | v0.32.0/v0.33.0 |           |

### Carbon emissions data source
Currently the best option is to create estimates based on the methodology used in [Boaviztapi](https://github.com/Boavizta/boaviztapi).

[Try out Boaviztapi on the Datavizta demo website](https://datavizta.boavizta.org/cloudimpact).

#### Licensing
Boaviztapi is licensed under [`GNU Affero General Public License v3.0`](https://github.com/Boavizta/boaviztapi/blob/main/LICENSE). Therefore, as far as I know, we must license their data under the same license if used in the Karpenter repository.

#### Limitations
There is a discrepancy between the available instances known to Karpenter and instances know to Boaviztapi. This means that as it is right now, it is not possible to get carbon emissions data for all instances types. This is mostly the case for new instance types such as m7g. Around 290 out of 700 instance types are missing data. See full comparison in [this Gist](https://gist.github.com/JacobValdemar/e1342013c0f5c980126f6a1feb66b4a1).

I will attempt to eleminate this discrepancy, but it might not be possible. It will probably not always be possible to have an updated list of estimated carbon emissions for all instances as AWS continues to release new instance types. We should consider what to do with instance types that we do not have carbon emission estimates for.

Approaches to handle this:
1. Estimate extremely high emissions to effectively filter out unknown instance types (recommended)
2. Estimate zero emissions

### Launch strategy
To enable emission based priotization, the launch strategy should be changed from `lowest-price` to `prioritized`.

### Changes to consolidation (karpenter-core)
Single Machine Consolidation (`singlemachineconsolidation.go`) and Multi Machine Consolidation (`multimachineconsolidation.go`) as well as `consolidation.go` is currently consolidating nodes to reduce costs. We want to change this when Carbon Aware is enabled. They should consolidate to minimize carbon emissions. 

### Changes to Provisioning
Currently, provisioning (roughly) filter instances based on requirements, sort instances by price, and launch the cheapest instance. We want to change this when Carbon Aware is enabled. It should sort instances by carbon emissions and launch the instance which has the lowest Global Warming Potential[^1].

### Option 1: Use Carbon Aware provisioning and consolidation methods

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
In `karpenter-core`, create a new method `types.go/OrderByCarbonEmissions` and use that in `nodeclaimtemplate.go/ToMachine` and `nodeclaimtemplate.go/ToNodeClaim` instead of `types.go/OrderByPrice` when Carbon Aware is enabled.

In `karpenter`, create a new method `CarbonAwareCreate` in `pkg/providers/instance/instance.go` that is used in `pkg/cloudprovider/cloudprovider.go/Create` instead of `pkg/providers/instance/instance.go/Create` when Carbon Aware is enabled.

#### Considerations
1. 👍 Current consolidation methods are unaffected.
1. 👎 There might be copy-paste of code from the original consolidation methods to the carbon aware consolidators.

### Option 2: Use Carbon Aware filtering/sorting methods

#### Consolidation
Create carbon aware implementations of low-level functions like `filterByPrice`, `filterOutSameType`, `getCandidatePrices`, etc. that are used when Carbon Aware is enabled. Usage of aforementioned functions might assume that it is price that they are getting, but in reality it is data about carbon emissions.

#### Provisioning
Use same changes to provisioning as in [option 1](#option-1-use-carbon-aware-provisioning-and-concolidation-methods).

#### Considerations
1. 👍 Less code copy-paste.
1. 👍 Improvements to original consolidation methods also improve the Carbon Aware feature.
1. 👎 Has a risk of breaking undocumented invariants.
1. 👎 Adds complexity to the original consolidation methods.

### Option 3: Override instance price with carbon price (recommended)
Minimize carbon emissions by defining a price per kgCO₂e and override the instance price with the carbon price (USD/kgCO₂e). Using the `prioritized` launch strategy, carbon emissions will be minimized during provisioning. Consolidation will unknowingly consolidate to minimize carbon emissions.

The carbon price will depend on on `region` and `instanceType` and assume constant resource utilization (e.g. always 80% utilization). The carbon price will be generated in a "hack" and included as consts (same method as used for generating initial pricing[^2]). The carbon price / emission estimates can be updated with new versions.

Another feature (added later) can be to add carbon price to instance price to simulate a [carbon tax](https://en.wikipedia.org/wiki/Carbon_tax). Administrators could configure a custom carbon price or use a default.

#### Considerations
1. 👍 Change is constrained to the pricing domain, so most of Karpenter's logic remains unaffected.
1. 👍👍 A simulated carbon tax could be appealing for *Beta* or *General Availability*[^3] as it combines the real price with the carbon price.
1. 👎 Adds complexity to the *price* concept. Price is not *just* price, but rather becomes an optimization function.
1. 👎 Depending on implementation, the `karpenter_cloudprovider_instance_type_price_estimate` metric *may* represent more than just price when Carbon Aware is enabled.

### Option 4: Enable custom instance price overrides
Enable administrators to configure custom instance price overrides, e.g. in a ConfigMap. A configuration using emission factors (varying with region and instance type) masked as prices can be pre-generated. Administrators then copy-paste a Carbon Aware `priceOverride` into their environment.

```yaml
priceOverrides:
  - instanceType: "m5.large"
    region: "eu-west-1"
    capacityType: OnDemand
    price: 0.007712
  - instanceType: "m5.xlarge"
    region: "eu-west-1"
    capacityType: OnDemand
    price: 0.015424
```

<details>
<summary>Alternative interface</summary>

Alternatively, a more flexible interface could be:

```yaml
priceModification:
  operator: Add # Add or Override
  modifications:
    - instanceType: "m5.large"
      region: "eu-west-1"
      capacityType: OnDemand
      price: 0.007712
    - instanceType: "m5.xlarge"
      region: "eu-west-1"
      capacityType: OnDemand
      price: 0.015424
```
</details>

A ConfigMap with price overrides for all combinations of instance types and regions will be very huge. 632 instances * 29 regions = 18,328 pairs. Four lines per pair gives a file with 73,312 lines. The file/configmap will approximately have a size of 2 MB. That exceeds the [`1 MiB` limit on ConfigMap size in Kubernetes](https://kubernetes.io/docs/concepts/configuration/configmap/#motivation).

#### Considerations
1. 👍 Simple solution.
1. 👍 Can be used for other purposes.
2. 👎👎 ConfigMap cannot contain all data.
3. 👎 Hard to discover the carbon aware "feature".
4. 👎 Carbon emission price cannot be combined with actual price.
5. 👎 Carbon emissions are completely static without possibility to improve it in the future.
7. 👎 Feature can not be enabled as a toggle.
8. 👎 Depending on implementation, the `karpenter_cloudprovider_instance_type_price_estimate` metric *may* represent more than just price when Carbon Aware is enabled.

[^1]: The potential impact of greenhouse gases on global warming. Measured in terms of CO₂e.
[^2]: See [prices_gen.go](/hack/code/prices_gen.go) and [zz_generated.pricing.go](/pkg/providers/pricing/zz_generated.pricing.go)
[^3]: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-stages