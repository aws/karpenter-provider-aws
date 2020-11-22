# Frequently Asked Questions

1. **Why should I use Karpenter?**
Karpenter enables Kubernetes users to maximize resource utilization and improve availability for their clusters without requiring them to manually allocate or over-provision resources. Users can choose the metrics they want to drive the amount of compute resources allocated for their cluster, letting them scale their clusters independently, ahead-of, or in-concert with the scale of their applications. Users can configure scaling across multiple compute options and Karpenter offers straightforward and highly customizable options for scaling that are defined with configuration files, so they can be easily shared and implemented across multiple clusters. Karpenter runs as a set of linked components within a Kubernetes cluster, which allows the system to make fast, linear time scaling decisions for any size of cluster.

2. **How do I start using Karpenter?**
Check out our [Getting Started documentation](/README.md#getting-started) for installation instructions and common usage patterns.

3. **How does Karpenter compare to the Kubernetes cluster autoscaler?**
The Kubernetes cluster autoscaler reacts to the number of pods running on the cluster and tightly coupled to the type of compute used as well as the number of pods running. This means that cluster autoscaler will not scale a cluster until more pods are running and means that for workloads where you desire to scale on a schedule or over-provision, you need to implement work arounds such as ‘Hollow Pods’ that essentially hack cluster autoscaler. Karpenter is more flexible. You can apply a traditional pending pods metric to some of your node groups, while using different metrics like reserved capacity to scale other node groups. This gives Karpenter the flexibility to implement sophisticated scaling behavior to accommodate a wide variety of use cases.

4. **Where can I use Karpenter?**
Karpenter works with any Kubernetes cluster running in any environment. You can use Karpenter with Kubernetes clusters in the cloud and on premises.

5. **How does Karpenter work?**
Karpenter is an open, metrics-driven autoscaling system. There are four logical components: 1/ metrics producer which outputs metrics that can be used to drive scaling, 2/ metrics server which aggregates and stores scaling metric data from each producer, 3/ autoscaler which contains the logic for scaling including metric tracking strategies and scaling policies, and 4/ replica controller which changes the number of desired replicas for a unit of compute. These components are able to be implemented together or flexibly in combination with other systems such as KEDA. See appendix for more information about the Karpenter system architecture.

6. **Is Karpenter a node autoscaler only? Are there plans to make it extensible for workloads as well?**
At launch, we plan to build replica controllers to drive node scaling for Kubernetes clusters. That said, Karpenter has an open design can be used to scale anything that implements the scale sub-resource. This includes multiple Kubernetes resource controllers and even cloud provider resources that are controlled from the Kuberentes API (such as with [ACK](https://github.com/aws/aws-controllers-k8s)).

7. **Can I define my own metrics?**
Yes! You can write your own metrics producers for any metric you want to use to scale your cluster.

8. **How does scaling work in Karpenter?**
Karpenter manages scaling based on the principals of proportional control. This means that Karpenter attempts to maintain a desired number of compute resources in proportion to a scaling metric that you define. Similar to how you set a minimum and maximum temperature for your house’s thermostat, you can set separate scaling rules for these proportions with relation to both scaling up and scaling down. Each scaling rule includes a scaling policy that allows you to define which metric to scale off of and how to scale based on that metric. In Karpenter you can define multiple metrics, and multiple scaling policies and apply these to separate node groups. In this way, Karpenter can be as simple, or as complex, as your use case dictates. If you want to simply scale up all nodes based on the number of items in a queue or inbound connection requests at a load balancer, you can do that. If you want to scale up certain node groups based on CPU utilization and only scale down if traffic to the website drops below a particular threshold, you can do that also.

9. **Some cloud providers have existing managed scaling systems, such as predictive scaling. Can Karpenter use those?**
Yes. Each component of Karpenter is decoupled so that any can be swapped out to take advantage of existing managed systems. This means that any existing predictive scaling system can be integrated into Karpenter by using that scaling system as a metrics source. Additionally, Karpenter can feed data to these systems by sending them scale decisions as a replica target. At the extreme, this could mean using Karpenter as an API standard and having all functionality fulfilled by other systems.

10. **What kinds of metric targets can I use to setup scaling policies?**
Karpenter can scale using three types of metric targets: value, average-value, and utilization. Value and utilization let you drive scaling based on the metric signal and existing replicas. Average value lets you track scaling to a desired input metric value and is independent of replicas. You can use these interchangeably with metrics signals and Karpenter scaling policies to drive different scaling behavior.

11. **Can Karpenter scale from/to 0?**
Because average value metric target type is independent of the number of existing replicas in a cluster, you can use this metric target type to drive scaling decisions when replicas are 0. Furthermore, Karpenter lets you define multiple metrics signals in a single policy, allowing you to scale from zero using an average value metric and then beyond that using a completely different metric.

12. **Does Karpenter work with multiple types of nodes?**
Karpenter allows you to target specific policies for specific node groups. For example, you can scale one group by one node for every 1k inbound connection requests and another group by two nodes for every 1k inbound connection requests. When Karpenter sees the inbound connections increase, it will request the appropriate node group to scale by the relative amount you define. This pattern works with different sizes of node groups, but also can be extended to different types of nodes like ARM and Spot where you need the control over which applications and scenarios scale these groups. Karpenter lets you directly define the relationship between the scaling metric and what is scaled. This gives you fine-grained control in how to scale different types of nodes across your cluster and still allows you to apply globally computed metrics to a set of node groups.

13.	**Does Karpenter work with EC2 Spot?**
Yes. Karpenter allows you to specify fine grained policies on any node group. You can configure multiple node groups to scale with different or the same metrics, depending on your use case. One way to do this with spot is to configure a `capacityReservation` metric for two node groups with spot instances of different instance types. As the scheduler fills the nodes with incoming pods, the node groups will scale out. If one of the instances types becomes unavailable, the other node group will continue to scale.

14.	**Does Karpenter respect pod disruption budgets?**
Karpenter does not include an integration to Kubernetes lifecycle hooks in order to drain nodes during scaling. However, Karpenter does allow you to connect resources like an EKS managed node group (MNG) to the replica controller. Resources such as MNG have existing integrations to Kubernetes lifecycle events to ensure graceful scale down events.

15.	**How does Karpenter work with Prometheus?**
Karpenter uses promql queries for its HorizontalAutoscaler. Any metrics available in Prometheus can be used to autoscale a resource. For example, you can use Karpenter's MetricsProducer resource, kube-state-metrics, or any custom code that exposes metrics to Prometheus in your scaling policies.

16.	**Metrics-driven open source autoscaling systems like HPA and KEDA exist today. How is Karpenter different?**
Systems including HPA are similar to Karpenter, but designed to manage pod scaling with the assumption that a reactive node scaling system will ensure enough compute is available for the cluster. Karpenter takes a very similar approach to these exiting metrics-driven systems, and uses many of the same principals, applying them to node scaling.

17.	**Cluster Autoscaler has worked just fine for my clusters, why should I use Karpenter?**
Cluster autoscaler works well for a number of common use cases. However, some use cases such as scheduled scaling or batch processing, you have to do a lot of extra work or accept the performance and resource inefficiencies created by the architecture of Cluster Autoscaler reacting to pending pods. If you have a mix of use cases in your organization, this means that some users have a fundamentally different architecture and approach to scaling their clusters based on what they are doing. Karpenter allows standardization in how scaling works across your entire organization and for all use cases. Its flexibility lets you optimize your cluster scaling to meet any Kubernetes application use case while reducing implementation complexities and differences that cause delays and take significant extra work on behalf of some teams.

18. **Metrics will be polled periodically to calculate the desired replicas. Is it possible to configure the polling period?**
The default polling period is 10 seconds, though the user can configure this in their HorizontalAutoscaler policy.

19.	**Is this just an AWS project?**
This is an AWS initiated project, but we intend our working group to grow to members across the Kubernetes community. We welcome and encourage anyone to join us! See [contributing](./CONTRIBUTING.md).
