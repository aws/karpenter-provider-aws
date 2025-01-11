---
title: "Managing AMIs"
linkTitle: "Managing AMIs"
weight: 10
description: >
  Task for managing AMIs in Karpenter
---

{{% alert title="Important" color="warning" %}}
Karpenter __heavily recommends against__ opting-in to use an `amiSelectorTerm` with `@latest` unless you are doing this in a pre-production environment or are willing to accept the risk that a faulty AMI may cause downtime in your production clusters. In general, if using a publicly released version of a well-known AMI type (like AL2, AL2023, or Bottlerocket), we recommend that you pin to a version of that AMI and deploy newer versions of that AMI type in a staged approach when newer patch versions are available.

```yaml
amiSelectorTerms:
  - alias: al2023@v20240807
```

More details are described in [Controlling AMI Replacement]({{< relref "#controlling-ami-replacement" >}}) below.
{{% /alert %}}

Understanding how Karpenter assigns AMIs to nodes can help ensure that your workloads will run successfully on those nodes and continue to run if the nodes are upgraded to newer AMIs.
Below we describe how Karpenter assigns AMIs to nodes when they are first deployed and how newer AMIs are assigned later when nodes are spun up to replace old ones.
Later, it describes the options you have to assert control over how AMIs are used by Karpenter for your clusters.

Features for managing AMIs described here should be considered as part of the larger upgrade policies that you have for your clusters.
See [How do I upgrade an EKS Cluster with Karpenter]({{< relref "../faq/#how-do-i-upgrade-an-eks-cluster-with-karpenter" >}}) for details on this process. 

## How Karpenter assigns AMIs to nodes

Here is how Karpenter assigns AMIs nodes:

* When you create an `EC2NodeClass`, you are required to specify [`amiSelectorTerms`]({{< relref "../concepts/nodeclasses/#specamiselectorterms" >}}). [`amiSelectorTerms`]({{< relref "../concepts/nodeclasses/#specamiselectorterms" >}}) allow you to select on AMIs that can be spun-up by this EC2NodeClass based on tags, id, name, or an alias.
* If you specify an `amiSelectorTerm` with an `alias` set to `@latest` e.g. `al2023@latest`, `al2@latest`, `bottlerocket@latest`, any time Karpenter spins up a new node, it uses the latest release for that AMI type.
* When a node is replaced, Karpenter checks to see if a newer AMI is available based on your `amiSelectorTerms`. If a newer AMI is available, Karpenter will automatically use the new AMI to spin up the new node. In particular, if you are using an `alias` set to `@latest`, you may get a new AMI deployed to your environment without having properly tested it.
  * Karpenter automatically marks nodes as `Drifted` (determining that the node needs to be replaced) when the latest AMI that matches your `amiSelectorTerms` does not match the AMI assigned to the node.

You can manually delete a node managed by Karpenter, which will cause the replacement behavior mentioned above to take effect. Karpenter will also automatically replace your nodes when the node meets a certain replacement condition. These conditions include: [**Expiration**]({{< relref "../concepts/disruption/#expiration" >}}) (if node expiry is set, the node is marked for deletion at a certain time after the node is created), [**Consolidation**]({{< relref "../concepts/disruption/#consolidation" >}}) (if a node is empty of workloads, or deemed to be inefficiently running workloads, nodes can be deleted and more appropriately featured nodes are brought up to consolidate workloads), [**Drift**]({{< relref "../concepts/disruption/#drift" >}}) (nodes are set for deletion when they drift from the desired state of the `NodeClaims` and new nodes are brought up to replace them), and [**Interruption**]({{< relref "../concepts/disruption/#interruption" >}}) (nodes are sometimes involuntarily disrupted by things like Spot interruption, health changes, and instance events, requiring new nodes to be deployed).

See [**Automated Methods**]({{< relref "../concepts/disruption/#automated-methods" >}}) for details on how Karpenter uses these automated actions to replace nodes.

{{% alert title="Important" color="warning" %}}
If you are new to Karpenter, you should know that the behavior described here is different than you get with Managed Node Groups (MNG). MNG will always use the assigned AMI when it creates a new node and will never automatically upgrade to a new AMI when a new node is required. See [Updating a Managed Node Group](https://docs.aws.amazon.com/eks/latest/userguide/update-managed-node-group.html) to see how you would manually update MNG to use new AMIs.
{{% /alert %}}

## Controlling AMI Replacement

Karpenter's automated node replacement functionality in tandem with the `EC2NodeClass` gives you a lot of flexibility to control the desired state of nodes on your cluster. For example, you can opt-in to AMI auto-upgrades using `alias` set to `@latest`; however, this has to be weighed heavily against the risk of newer versions of an AMI breaking existing applications on your cluster. Alternatively, you can choose to pin your AMIs in your production clusters to avoid the risk of breaking changes; however, this has to be weighed against the management cost of testing new AMIs in pre-production and keeping up with the latest AMI versions.

Karpenter offers you various controls to ensure you don't take on too much risk as you rollout new versions of AMIs to your production clusters. Below shows how you can use these controls:

* [Pinning AMIs]({{< relref "#pinning-amis" >}}): If workloads require a particluar AMI, this control ensures that it is the only AMI used by Karpenter. This can be used in combination with [Testing AMIs]({{< relref "#testing-amis" >}}) where you lock down the AMI in production, but allow the newest AMIs in a test cluster while you test your workloads before upgrading production.
* [Testing AMIs]({{< relref "#testing-amis" >}}): The safest way for ensuring that a new AMI doesn't break your workloads is to test it before putting it into production. This takes the most effort on your part, but most effectively models how your workloads will run in production, allowing you to catch issues ahead of time. Note that you can sometimes get different results from your test environment when you roll a new AMI into production, since issues like scale and other factors can elevate problems you might not see in test. Combining this with other controls like [Using Disruption Budgets]({{< relref "#using-disruption-budgets" >}}) can allow you to catch problems before they impact your whole cluster.
* [Using Disruption Budgets]({{< relref "#using-disruption-budgets" >}}): This option can be used as a way of mitigating the scope of impact if a new AMI causes problems with your workloads. With Disruption budgets you can slow the pace of upgrades to nodes with new AMIs or make sure that upgrades only happen during selected dates and times (using `schedule`). This doesn't prevent a bad AMI from being deployed, but it allows you to control when nodes are upgraded, and gives you more time to respond to rollout issues.

## Controls

The following lays out the controls you have to impact Karpenter’s behavior as it relates to how nodes are created and AMIs are consumed.

### Pinning AMIs

When you configure the [**EC2NodeClass**]({{< relref "../concepts/nodeclasses" >}}), you are required to configure which AMIs you want Karpenter to select on using the `amiSelectorTerms` field. When pinning to a specific `id`, `name` or an `alias` that contains a fixed version, Karpenter will only select on a single AMI and won't automatically upgrade your nodes to a new version of an AMI. This prevents a new and potentially untested AMI from replacing existing nodes when those nodes are terminated.
).
These examples show three different ways to identify the same AMI:

```yaml
# Using alias
amiSelectorTerms:
- alias: al2023@v20240219
```

```yaml
# Using name
amiSelectorTerms:
- name: al2023-ami-2023.3.20240219.0-kernel-6.1-x86_64
```

```yaml
# Using id
amiSelectorTerms:
- id: ami-052c9ea013e6e3567
```

See the [**spec.amiSelectorTerms**]({{< relref "../concepts/nodeclasses/#specamiselectorterms" >}}) section of the NodeClasses page for details.
Keep in mind, that this could prevent you from getting critical security patches when new AMIs are available, but it does give you control over exactly which AMI is running.

### Testing AMIs

Instead of avoiding AMI upgrades, you can set up test clusters where you can try out new AMI releases before they are put into production. For example, you could have:

* **Test clusters**: On lower environment clusters, you can run the latest AMIs e.g. `al2023@latest`, `al2@latest`, `bottlerocket@latest`, for your workloads in a safe environment. This ensures that you get the latest patches for AMIs where downtime to applications isn't as critical and allows you to validate patches to AMIs before they are deployed to production.

* **Production clusters**: After you've confirmed that the AMI works in your lower environments, you can pin the latest AMIs to be deployed in your production clusters to roll out the AMI. Refer to [Pinning AMIs]({{< relref "#pinning-amis" >}}) for how to choose a particular AMI by `alias`, `name` or `id`. Remember that it is still best practice to gradually roll new AMIs into your cluster, even if they have been tested. So consider implementing that for your production clusters as described in [Using Disruption Budgets]({{< relref "#using-disruption-budgets" >}}).

### Using Disruption Budgets

To reduce the risk of entire workloads being immediately degraded when a new AMI is deployed, you can enable Karpenter's [**Node Disruption Budgets**]({{< relref "#node-disruption-budgets " >}}) as well as ensure that you have [**Pod Disruption Budgets**]({{< relref "#pod-disruption-budgets " >}}) configured for applications on your cluster. Below provides more details on how to configure each.

#### Node Disruption Budgets

[Disruption Budgets]({{< relref "../concepts/disruption/#disruption-budgets " >}}) limit when and to what extent nodes can be disrupted. You can prevent disruption based on nodes (a percentage or number of nodes that can be disrupted at a time) and schedule (excluding certain times from disrupting nodes).
You can set Disruption Budgets in a `NodePool` spec. Here is an example:

```yaml
disruption:
  budgets:
  - nodes: 15%
  - nodes: "3"
  - nodes: "0"
    schedule: "0 9 * * sat-sun"
    duration: 24h
  - nodes: "0"
    schedule: "0 17 * * mon-fri"
    duration: 16h
    reasons:
      - Drifted
```

Settings for budgets in the above example include the following:

* **Percentage of nodes**: From the first `nodes` setting, only `15%` of the NodePool’s nodes can be disrupted at a time.
* **Number of nodes**: The second `nodes` setting limits the number of nodes that can be disrupted at a time to `3`.
* **Schedule**: The third `nodes` setting uses schedule to say that zero disruptions (`0`) are allowed starting at 9am on Saturday and Sunday and continues for 24 (fully blocking disruptions all day).
The format of the schedule follows the `crontab` format for identifying dates and times.
See the [crontab](https://man7.org/linux/man-pages/man5/crontab.5.html) page for information on the supported values for these fields.
* **Reasons**: The fourth `nodes` setting uses `reasons` which implies that this budget only applies to the `Drifted` disruption condition. This setting uses schedule to say that zero disruptions (`0`) are allowed starting at 5pm on Monday, Tuesday, Wednesday, Thursday, and Friday and continues for 16h (effectively blocking rolling nodes due to drift outside of working hours).

As with all disruption settings, keep in mind that avoiding updated AMIs for your nodes can result in not getting fixes for known security risks and bugs.
You need to balance that with your desire to not risk breaking the workloads on your cluster.

#### Pod Disruption Budgets

[Pod Disruption Budgets](https://kubernetes.io/docs/tasks/run-application/configure-pdb/#specifying-a-poddisruptionbudget) allow you to describe how much disruption an application can tolerate before it begins to become unhealthy. This is critical to configure for Karpenter, since Karpenter uses this information to determine if it can continue to replace nodes. Specifically, if replacing a node would cause a Pod Disruption Budget to be breached (for graceful forms of disruption e.g. Drift or Consolidation), Karpenter will not replace the node.

In a scenario where a faulty AMI is rolling out and begins causing downtime to your applications, configuring Pod Disruption Budgets is critical since this will tell Karpenter that it must stop replacing nodes until your applications become healthy again. This prevents Karpenter from deploying the faulty AMI throughout your cluster, reduces the imact the AMI has on your production applications, and gives you manually intervene in the cluster to remediate the issue.

## Follow-up

The Karpenter project continues to add features to give you greater control over AMI upgrades on your clusters.
If you have opinions about features you would like to see to manage AMIs with Karpenter, feel free to enter a Karpenter [New Issue](https://github.com/aws/karpenter-provider-aws/issues/new/choose).
