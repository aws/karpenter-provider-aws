---
title: "Managing AMIs"
linkTitle: "Managing AMIs"
weight: 10
description: >
  Task for managing AMIs in Karpenter
---

Understanding how Karpenter assigns AMIs to nodes can help ensure that your workloads will run successfully on those nodes and continue to run if the nodes are upgraded to newer AMIs.
Below we describe how Karpenter assigns AMIs to nodes when they are first deployed and how newer AMIs are assigned later when nodes are spun up to replace old ones.
Later, it describes the options you have to assert control over how AMIs are used by Karpenter for your clusters.

Features for managing AMIs described here should be considered as part of the larger upgrade policies that you have for your clusters.
See [How do I upgrade an EKS Cluster with Karpenter]({{< relref "../faq/#how-do-i-upgrade-an-eks-cluster-with-karpenter" >}}) for details on this process. 

## How Karpenter assigns AMIs to nodes

Here is how Karpenter assigns AMIs nodes:

* When you create an `EC2NodeClass`, you are required to set the family of AMIs to use. For example, for the AL2 family, you would set `amiFamily: AL2`.
* With that `amiFamily` set, any time Karpenter spins up a new node, it uses the latest [Amazon EKS optimized Amazon Linux 2 AMIs](https://docs.aws.amazon.com/eks/latest/userguide/eks-optimized-ami.html) release.
* Later, if an existing node needs to be replaced, Karpenter checks to see if a newer AMI in the AL2 family is available and automatically uses the new AMI instead to spin up the new node. In other words, you may automatically get an AMI that you have not tested with your workloads.

You can manually delete a node managed by Karpenter, which will cause the default behavior just described to take effect.
However, there are situations that will cause node replacements with newer AMIs to happen automatically.
These include: Expiration (if node expiry is set, the node is marked for deletion at a certain time after the node is created), [**Consolidation**]({{< relref "../concepts/disruption/#consolidation" >}}) (if a node is empty of workloads, or deemed to be inefficiently running workloads, nodes can be deleted and more appropriately featured nodes are brought up to consolidate workloads), [Drift]({{< relref "../concepts/disruption/#drift" >}}) (nodes are set for deletion when they drift from the desired state of the `NodeClaims` and new nodes are brought up to replace them), and [Interruption]({{< relref "../concepts/disruption/#interruption" >}}) (nodes are sometimes involuntarily disrupted by things like Spot interruption, health changes, and instance events, requiring new nodes to be deployed).

See [**Automated Methods**]({{< relref "../concepts/disruption/#automated-methods" >}}) for details on how Karpenter uses these automated actions to replace nodes.

With these types of automated updates in place, there is some risk that the new AMI being used when replacing instances will introduce some regressions or bugs that cause your workloads to be degraded or fail altogether.
The options described below tell you how to take more control over the ways in which Karpenter selects AMIs for your nodes.

{{% alert title="Important" color="warning" %}}
If you are new to Karpenter, you should know that the behavior described here is different than you get with Managed Node Groups (MNG). MNG will always use the assigned AMI when it creates a new node and will never automatically upgrade to a new AMI when a new node is required. See [Updating a Managed Node Group](https://docs.aws.amazon.com/eks/latest/userguide/update-managed-node-group.html) to see how you would manually update MNG to use new AMIs.
{{% /alert %}}

## Choosing AMI options
One of Karpenter's greatest assets is its ability to provide the right node at the right time, with little intervention from the person managing the cluster.
Its default behavior of using a later AMI if one becomes available in the selected family means you automatically get the latest security fixes and features.
However, with this comes the risk that the new AMI could break or degrade your workloads.

As the Karpenter team looks for new ways to manage AMIs, the options below offer some means of reducing these risks, based on your own security and ease-of-use requirements.
Here are the advantages and challenges of each of the options described below:

* [Option 1]({{< relref "#option-1-manage-how-amis-are-tested-and-rolled-out" >}}) (Test AMIs): The safest way, and the one we recommend, for ensuring that a new AMI doesn't break your workloads is to test it before putting it into production. This takes the most effort on your part, but most effectively models how your workloads will run in production, allowing you to catch issues ahead of time. Note that you can sometimes get different results from your test environment when you roll a new AMI into production, since issues like scale and other factors can elevate problems you might not see in test. So combining this with other options, that do things like slow rollouts, can allow you to catch problems before they impact your whole cluster.
* [Option 2]({{< relref "#option-2-lock-down-which-amis-are-selected" >}}) (Lock down AMIs): If workloads require a particluar AMI, this option can make sure that it is the only AMI used by Karpenter. This can be used in combination with Option 1, where you lock down the AMI in production, but allow the newest AMIs in a test cluster while you test your workloads before upgrading production. Keep in mind that this makes upgrades a manual process for you.
* [Option 3]({{< relref "#option-3-control-the-pace-of-node-disruptions" >}}) ([Disruption budgets]({{< relref "../concepts/disruption/" >}})): This option can be used as a way of mitigating the scope of impact if a new AMI causes problems with your workloads. With Disruption budgets you can slow the pace of upgrades to nodes with new AMIs or make sure that upgrades only happen during selected dates and times (using `schedule`). This doesn't prevent a bad AMI from being deployed, but it allows you to control when nodes are upgraded, and gives you more time respond to rollout issues.

## Options

The following lays out the options you have to impact Karpenter’s behavior as it relates to how nodes are created and AMIs are consumed.

### Option 1: Manage how AMIs are tested and rolled out

Instead of just avoiding AMI upgrades, you can set up test clusters where you can try out new AMI releases before they are put into production.
For example, you could have:

* **Test clusters**: On lower environment clusters, you can run the latest AMIs for your workloads in a safe environment. The `EC2NodeClass` for these clusters could be set with a chosen `amiFamily`, but no `amiSelectorTerms` set. For example, the `NodePool` and `EC2NodeClass` could begin with the following:

  ```yaml
  apiVersion: karpenter.sh/v1beta1
  kind: NodePool
  metadata:
    name: default
  spec:
    template:
      spec:
        nodeClassRef:
          apiVersion: karpenter.k8s.aws/v1beta1
          kind: EC2NodeClass
          name: default
  ---
  apiVersion: karpenter.k8s.aws/v1beta1
  kind: EC2NodeClass
  metadata:
    name: default
  spec:
    # The latest AMI in this family will be used
    amiFamily: AL2
  ```
* **Production clusters**: After you've confirmed that the AMI works in your lower environments, you can pin the latest AMIs to be deployed in your production clusters to roll out the AMI. One way to do that is to use `amiSelectorTerms` to set the tested AMI to be used in your production cluster. Refer to Option 2 for how to choose a particular AMI by `name` or `id`. Remember that it is still best practice to gradually roll new AMIs into your cluster, even if they have been tested. So consider implementing that for your production clusters as described in Option 3.

### Option 2: Lock down which AMIs are selected

Instead of letting Karpenter always run the latest AMI, you can change Karpenter’s default behavior.
When you configure the [**EC2NodeClass**]({{< relref "../concepts/nodeclasses" >}}), you can set a specific AMI that you want Karpenter to always choose, using the `amiSelectorTerms` field.
This prevents a new and potentially untested AMI from replacing existing nodes when those nodes are terminated.

With the `amiSelectorTerms` field in an `EC2NodeClass`, you can set a specific AMI for Karpenter to use, based on AMI name or id (only one is required).
These examples show two different ways to identify the same AMI:

```yaml
amiSelectorTerms:
- tags:
    karpenter.sh/discovery: "${CLUSTER_NAME}"
    environment: prod
- name: al2023-ami-2023.3.20240219.0-kernel-6.1-x86_64
```

or

```yaml
amiSelectorTerms:
- tags:
    karpenter.sh/discovery: "${CLUSTER_NAME}"
    environment: prod
- id: ami-052c9ea013e6e3567
```

See the [**spec.amiSelectorTerms**]({{< relref "../concepts/nodeclasses/#specamiselectorterms" >}}) section of the NodeClasses page for details. 
Keep in mind, that this could prevent you from getting critical security patches when new AMIs are available, but it does give you control over exactly which AMI is running.


### Option 3: Control the pace of node disruptions

To reduce the risk of entire workloads being immediately degraded when a new AMI is deployed, you can enable Karpenter [**Disruption Budgets**]({{< relref "../concepts/disruption/#disruption-budgets " >}}).
Disruption Budgets limit when and to what extent nodes can be disrupted.
You can prevent disruption based on nodes (a percentage or number of nodes that can be disrupted at a time) and schedule (excluding certain times from disrupting nodes).
You can set Disruption Budgets in a `NodePool` spec.
Here is an example:

```yaml
disruption:
  consolidationPolicy: WhenEmpty
  expireAfter: 1440h
  budgets:
  - nodes: 15%
  - nodes: "3"
  - nodes: "0"
    schedule: "0 7 * * sat-sun"
    duration: 12h
```

The `disruption` settings define a few fields that indicate the state of a node that should be disrupted.
The `consolidationPolicy` field indicates that a node should be disrupted if the node is either underutilized (`WhenUnderutilized`) or not running any pods (`WhenEmpty`).
With `expireAfter` set to `1440` hours, the node expires after 60 days.
Extending those values causes longer times without disruption.

Settings for budgets in the above example include the following:

* **Percentage of nodes**: From the first `nodes` setting, only `15%` of the NodePool’s nodes can be disrupted at a time.
* **Number of nodes**: The second `nodes` setting limits the number of nodes that can be disrupted at a time to `3`.
* **Schedule**: The third `nodes` setting uses schedule to say that zero disruptions (`0`) are allowed starting at 7am on Saturday and Sunday and continues for 12 hours.
The format of the schedule follows the `crontab` format for identifying dates and times.
See the [crontab](https://man7.org/linux/man-pages/man5/crontab.5.html) page for information on the supported values for these fields.

As with all disruption settings, keep in mind that avoiding updated AMIs for your nodes can result in not getting fixes for known security risks and bugs.
You need to balance that with your desire to not risk breaking the workloads on your cluster.

## Follow-up

The Karpenter project continues to add features to give you greater control over AMI upgrades on your clusters.
If you have opinions about features you would like to see to manage AMIs with Karpenter, feel free to enter a Karpenter [New Issue](https://github.com/aws/karpenter-provider-aws/issues/new/choose).
