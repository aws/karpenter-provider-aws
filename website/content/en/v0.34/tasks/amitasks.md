---
title: "Managing AMIs"
linkTitle: "Managing AMIs"
weight: 10
description: >
  Tasks for managing AMIS in Karpenter
---

Understanding how Karpenter assigns AMIs to nodes can help ensure that your workloads will run successfully on those nodes and continue to run if the nodes are upgraded to newer AMIs.
Below we describe how Karpenter assigns AMIs to nodes when they are first deployed and how newer AMIs are assigned later when nodes are spun up to replace old ones.
Later, there are tasks that describe the ways that you can intervene to assert control over how AMIs are used by Karpenter for your clusters.

Features for managing AMIs described here should be considered as part of the larger upgrade policies that you have for your clusters.
See [How do I upgrade an EKS Cluster with Karpenter]({{< relref "../faq/#how-do-i-upgrade-an-eks-cluster-with-karpenter" >}}) for details on this process. 

## How Karpenter assigns AMIs to nodes by default

If you do nothing to modify how Karpenter handles AMIs, here is how Karpenter assigns AMIs nodes:

* When you create an `EC2NodeClass`, you are required to set the family of AMIs to use. For example, for the AL2 family, you would set `amiFamily: AL2`.
* With that `amiFamily` set, any time Karpenter needed to spin up a new node, it would use the latest AMI in the AL2 family.
* Later, if an existing node needs to be replaced, Karpenter checks to see if a newer AMI in the AL2 family is available and automatically uses the new AMI instead to spin up the new node. In other words, you may automatically get an AMI that you have not tested with your workloads.

You can manually delete a node managed by Karpenter, which will cause the default behavior just described to take effect.
However, there are situations that will cause node replacements with newer AMIs to happen automatically.
These include: 

* **Expiration**: If node expiry is set for a node, the node is marked for deletion after a certain time.
* [**Consolidation**]({{< relref "../concepts/disruption/#consolidation" >}}): If a node is empty of workloads, or deemed to be inefficiently running workloads, nodes can be deleted and more appropriately featured nodes are brought up to consolidate workloads.
* [**Drift**]({{< relref "../concepts/disruption/#drift" >}}): Nodes are set for deletion when they drift from the desired state of the `NodeClaim`s and new nodes are brought up to replace them.
* [**Interruption**]({{< relref "../concepts/disruption/#interruption" >}}): Nodes are sometimes involuntarily disrupted by things like Spot interruption, health changes, and instance events, requiring new nodes to be deployed.

See [**Automated Methods**]({{< relref "../concepts/disruption/#automated-methods" >}}) for details on how Karpenter uses these automated actions to replace nodes.

With these types of automated updates in place, there is some risk of a new AMI being brought up that introduces some incompatibilities or bugs that cause your workloads to be degraded or fail altogether.
The tasks described below tell you how to take more control over the ways in which Karpenter handles AMI assignments to nodes.

{{% alert title="Important" color="warning" %}}
If you are new to Karpenter, you should know that the behavior described here is different than you get with Managed Node Groups (MNG). MNG will always use the assigned AMI when it creates a new node and will never automatically upgrade to a new AMI when a new node is required. See [Updating a Managed Node Group](https://docs.aws.amazon.com/eks/latest/userguide/update-managed-node-group.html) to see how you would manually update MNG to use new AMIs.
{{% /alert %}}

## Choosing AMI tasks
One of Karpenter's greatest assets is its ability to provide the right node at the right time, with little intervention from the person managing the cluster.
Its default behavior of using a later AMI if one becomes available in the selected family means you automatically get the latest security fixes and features.
However, with this comes the risk that the new AMI could break or degrade your workloads.

As the Karpenter team looks for new ways to manage AMIs, the tasks below offer some means of reducing these risks, based on your own security and ease-of-use requirements.
Here are the advantages and challenges of each of the tasks described below:

* Task 1 (Test AMIs): The safest way, and the one we recommend, for ensuring that a new AMI doesn't break your workloads is to test it before putting it into production. This takes the most effort on your part, but can reduce the risk of failed workloads in production. Note that you can sometimes get different results from your test environment when you roll a new AMI into production, since issues like scale and other factors can elevate problems you might not see in test. So combining this with other tasks, that do things like slow rollouts, can allow you to catch problems before they impact your whole cluster.
* Task 2 (Lock down AMIs): If workloads require a particluar AMI, this task can make sure that it is the only AMI used by Karpenter. This can be used in combination with Task 1, where you lock down the AMI in production, but allow the newest AMIs in a test cluster while you test your workloads before upgrading production. Keep in mind that this makes upgrades a manual process for you.
* Task 3 (Disruption budgets): This task can be used as a way of preventing a major problem if a new AMI causes problems with your workloads. With Disruption budgets you can slow the pace of upgrades to nodes with new AMIs or make sure that upgrades only happen during selected dates and times (using crontab). This doesn't prevent a bad AMI from being deployed, but it does give you time to respond if a few upgraded nodes at a time show some distress.
* Task 4 (Do not interrupt): While this task doesn't represent a larger solution to the problem, it gives you the opportunity to either prevent all nodes or a node running a particular workload from being upgraded. Note that these settings have no impact in cases where the node is not in control of its being removed (such as when the instance it is running on crashes or a Spot instance is reclaimed).

## Tasks

The following tasks let you have an on impact Karpenter’s behavior as it relates to how nodes are created and AMIs are consumed.

### Task 1: Manage how AMIs are tested and rolled out

Instead of just avoiding AMI upgrades, you can set up test clusters where you can try out new AMI releases before they are put into production.
For example, you could have:

* **Test clusters**: On these private clusters, you can run the latest AMIs for your workloads in a safe environment.
* **Production clusters**: When you feel that everything is working properly, you can set the latest AMIs to be deployed in your production clusters so they are note upgraded.

Remember that it is still best practice to gradually roll new AMIs into your cluster, even if they have been tested.

### Task 2: Lock down which AMIs are selected

Instead of letting Karpenter always run the latest AMI, you can change Karpenter’s default behavior.
When you configure the [**EC2NodeClass**]({{< relref "../concepts/nodeclasses" >}}), you can set a specific AMI that you want Karpenter to always choose, using the `amiSelectorTerms` field.
This prevents a new and potentially untested AMI from replacing existing nodes when those nodes are terminated.

With the `amiSelectorTerms` field in an `EC2NodeClass`, you can set a specific AMI for Karpenter to use, based on AMI name or id (only one is required).
These examples show two different ways to identify the same AMI:

```bash
amiSelectorTerms:
  - tags:
      karpenter.sh/discovery: "${CLUSTER_NAME}"
      environment: prod
  - name: al2023-ami-2023.3.20240219.0-kernel-6.1-x86_64
```

```bash
amiSelectorTerms:
  - tags:
      karpenter.sh/discovery: "${CLUSTER_NAME}"
      environment: prod
  - id: ami-052c9ea013e6e3567
```

See the [**spec.amiSelectorTerms**]({{< relref "../concepts/nodeclasses/#specamiselectorterms" >}}) section of the NodeClasses page for details. 
Keep in mind, that this could prevent you from getting critical security patches when new AMIs are available, but it does give you control over exactly which AMI is running.


### Task 3: Control the pace of node disruptions

To help prevent the possibilities of a new AMI being deployed to all your nodes and breaking all of your workloads, you can enable Karpenter [**Disruption Budgets**]({{< relref "../concepts/disruption/#disruption-budgets " >}}).
Disruption Budgets limit when and to what extent nodes can be disrupted.
You can prevent disruption based on nodes (a percentage or number of nodes that can be disrupted at a time) and schedule (excluding certain times from disrupting nodes).
You can set Disruption Budgets in a `NodePool` spec.
Here is an example:

```bash
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
* **Number of nodes**: The second `nodes` setting limits the number of nodes that can be disrupted at a time to `5`.
* **Schedule**: The third `nodes` setting uses schedule to say that zero disruptions (`0`) are allowed starting at 7am on Saturday and Sunday and continues for 12 hours.
The format of the schedule follows the `crontab` format for identifying dates and times.
See the [crontab](https://man7.org/linux/man-pages/man5/crontab.5.html) page for information on the supported values for these fields.

As with all disruption settings, keep in mind that avoiding updated AMIs for your nodes can result in not getting fixes for known security risks and bugs.
You need to balance that with your desire to not risk breaking the workloads on your cluster.

### Task 4: Prevent Karpenter from disrupting nodes

There are several ways you can prevent Karpenter from disrupting nodes that it manages, to mitigate the risk of an untested AMI from being deployed.

* **Set Pods to not allow disruption**: When you run pods from a Deployment spec, you can set `karpenter.sh/do-not-disrupt` to true on that Deployment.
This will prevent the node that pod is running on from being disrupted while the pod is running (see [**Pod-level Controls**]({{< relref "../concepts/disruption/#pod-level-controls" >}}) for details).
This can be useful for things like batch jobs, which you want to run to completion and never be moved.
For example:

    
```bash
    apiVersion: apps/v1
    kind: Deployment
    spec:
      template:
        metadata:
          annotations:
            karpenter.sh/do-not-disrupt: "true"
```

* **Set nodes to not allow disruption** In the NodePool spec, you can set `karpenter.sh/do-not-disrupt` to true.
This prevents any nodes created from the NodePool from being considered for disruption (see [**Example: Disable Disruption on a NodePool**]({{< relref "../concepts/disruption/#example-disable-disruption-on-a-nodepool" >}}) for details).
For example:

```bash
    apiVersion: karpenter.sh/v1beta1
    kind: NodePool 
    metadata:
      name: default
    spec:
      template:
        metadata:
          annotations: # will be applied to all nodes
            karpenter.sh/do-not-disrupt: "true"
```

Keep in mind that these are not permanent solutions and cannot prevent all node disruptions, such as disruptions resulting from failed node health checks or the instance running the node going down.
Using only the methods to prevent disruptions described here, you will not prevent new AMIs from being used if an unintended disruption of a node occurs, unless you have already locked down specific AMIs to use.
## Follow-up

The Karpenter project continues to add features to give you greater control over AMI upgrades on your clusters.
If you have opinions about features you would like to see to manage AMIs with Karpenter, feel free to enter a Karpenter [New Issue](https://github.com/aws/karpenter-provider-aws/issues/new/choose).

