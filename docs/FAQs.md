# Frequently Asked Questions

1.	**What are you launching today?**
Today we are launching a developer preview of Karpenter, a new open source autoscaling system for Kubernetes that helps maximize application availability for Kubernetes clusters without requiring you to manually allocate or over-provision compute resources. Karpenter allows you to use the metrics of your choice to drive the amount of compute resources allocated for a cluster, letting you scale the compute for the cluster independently, ahead-of, or in-concert with the scale of your Kubernetes applications. Karpenter works with any Kubernetes cluster running in any environment including cloud and on-premises environments.

2.	**Why should I use Karpenter?**
Karpenter enables Kubernetes users to maximize resource utilization and improve availability for their clusters without requiring them to manually allocate or over-provision resources. Users can choose the metrics they want to drive the amount of compute resources allocated for their cluster, letting them scale their clusters independently, ahead-of, or in-concert with the scale of their applications. Customers can configure scaling across multiple compute options and Karpenter offers straightforward and highly customizable options for scaling that are defined with configuration files, so they can be easily shared and implemented across multiple clusters. Karpenter runs as a set of linked components within a Kubernetes cluster, which allows the system to make fast, linear time scaling decisions for any size of cluster.

3.	**How do I start using Karpenter?**
To use Karpenter, first download and run the Karpenter service on your Kubernetes cluster. Next, apply a configuration file that includes the required Karpenter resources. The Karpenter project offers several example files that you can use as-is, or modify. For example, the queue-scaling configuration will monitor the length of an AWS SQS queue and proactively scale the compute provisioned for the cluster based on the number of jobs waiting to be processed or you can use custom Prometheus metrics that you define like customer sign-ups to scale ahead of an event. You can even define your own Karpenter metrics sources and autoscaling logic to customize when, why, and how to scale compute resources for your cluster. See our [Getting Started documentation](./docs/aws/README.md).

4.	**Where can I use Karpenter?**
Karpenter works with any Kubernetes cluster running in any environment. You can use Karpenter with Kubernetes clusters in the cloud and on premises, including with EKS Paris and ModelRocket clusters.

5.	**How does Karpenter work?**
Karpenter is an open, metrics-driven autoscaling system. There are four logical components: 1/ metrics producer which outputs metrics that can be used to drive scaling, 2/ metrics server which aggregates and stores scaling metric data from each producer, 3/ autoscaler which contains the logic for scaling including metric tracking strategies and scaling policies, and 4/ replica controller which changes the number of desired replicas for a unit of compute. These components are able to be implemented together or flexibly in combination with other systems such as KEDA. See appendix for more information about the Karpenter system architecture.

6.	**Is Karpenter a node autoscaler only? Are there plans to make it extensible for workloads as well?**
At launch, we plan to build replica controllers to drive node scaling for Kubernetes clusters. That said, Karpenter has an open design can be used to scale anything that implements the scale sub-resource. This includes multiple Kubernetes resource controllers and even cloud provider resources that are controlled from the Kuberentes API (such as with [ACK](https://github.com/aws/aws-controllers-k8s)).

7.	**What signals can I use to scale my cluster?**
At launch, Karpenter includes the ability to use Prometheus and Amazon SQS metrics sources for scaling. In the future, we plan to add support for other metrics sources. Because Karpenter is open source and extensible, you can write your own metrics source to use any signal that your applications need to scale.

8.	**Can I define my own signals to scale my cluster?**
Yes! You can write your own metrics source integrations for any metrics source you want to use to scale your cluster.

9.	**How does scaling work in Karpenter?**
Karpenter manages scaling based on the principals of proportional control. This means that Karpenter attempts to maintain a desired number of compute resources in proportion to a scaling metric that you define. Similar to how you set a minimum and maximum temperature for your house’s thermostat, you can set separate scaling rules for these proportions with relation to both scaling up and scaling down. Each scaling rule includes a scaling policy that allows you to define which metric to scale off of and how to scale based on that metric. In Karpenter you can define multiple metrics, and multiple scaling policies and apply these to separate node groups. In this way, Karpenter can be as simple, or as complex, as your use case dictates. If you want to simply scale up all nodes based on the number of items in a queue or inbound connection requests at a load balancer, you can do that. If you want to scale up certain node groups based on CPU utilization and only scale down if traffic to the website drops below a particular threshold, you can do that also.

10. **What scaling logic does Karpenter use?**
At launch, Karpenter’s default autoscaler uses a proportional control algorithm for scaling. In the future, we will consider add support for Proportional Integral Derivative (PID) scaling. PID is a common control algorithm with applications in thermostats and automobile cruise control that can capture historical changes and extrapolate the current rate of change to make accurate scaling decisions. Karpenter uses this algorithm to maintain scale in the same way cruise control in your car maintains speed, making constant changes relative to the state of the cluster to achieve the desired scale. In the future, we hope to introduce other autoscaler algorithms, such as ML-driven predictive scaling and because Karpenter is extensible, you can write and integrate any autoscaling algorithm as your use case dictates.

11.	**Some cloud providers have existing managed scaling systems, such as predictive scaling. Can Karpenter use those?**
Yes. Each component of Karpenter is decoupled so that any can be swapped out to take advantage of existing managed systems. This means that any existing predictive scaling system can be integrated into Karpenter by using that scaling system as a metrics source. Additionally, Karpenter can feed data to these systems by sending them scale decisions as a replica target. At the extreme, this could mean using Karpenter as an API standard and having all functionality fulfilled by other systems.

12.	**What kinds of metric targets can I use to setup scaling policies?**
Karpenter can scale using three types of metric targets: value, average-value, and utilization. Value and utilization let you drive scaling based on the metric signal and existing replicas. Average value lets you track scaling to a desired input metric value and is independent of replicas. You can use these interchangeably with metrics signals and Karpenter scaling policies to drive different scaling behavior.

13.	**Can Karpenter scale from/to 0?**
Because average value metric target type is independent of the number of existing replicas in a cluster, you can use this metric target type to drive scaling decisions when replicas are 0. Furthermore, Karpenter lets you define multiple metrics signals to drive scaling. This means you can define scale from or to 0 policies that use average value tracked metrics (such as queue length) and then allow for dynamic scaling in a non-0 state using value or utilization metrics (such as CPU utilization).

14.	**Does Karpenter work with multiple types of nodes?**
Karpenter allows you to target multiple node groups as part of a single instance. You can define specific scaling policies for each node group and set their relationship to the desired scale. For example, you can scale one group by one node for every 1k inbound connection requests and another group by two nodes for every 1k inbound connection requests. When Karpenter sees the inbound connections increase, it will request the appropriate node group to scale by the relative amount you define. This pattern works with different sizes of node groups, but also can be extended to different types of nodes like ARM and Spot where you need the control over which applications and scenarios scale these groups. Karpenter lets you directly define the relationship between the scaling metric and what is scaled. This gives you fine-grained control in how to scale different types of nodes across your cluster and still allows you to apply globally computed metrics to a set of node groups.

15.	**Does Karpenter work with EC2 Spot?**
Yes. In Karpenter you target different node groups/types with separate replica controllers. Each replica controller can have a separate proportional scaling policy that drives off of a common metric. This makes it possible for Karpenter to proportionally and accurately scale spot nodes based on the desired capacity for the cluster, even when those spot nodes are of different sizes or availabilities.

16.	**Does Karpenter respect pod disruption budgets?**
Karpenter does not include an integration to Kubernetes lifecycle hooks in order to drain nodes during scaling. However, Karpenter does allow you to connect resources like an EKS managed node group (MNG) to the replica controller. Resources such as MNG have existing integrations to Kubernetes lifecycle events to ensure graceful scale down events.

17.	**How does Karpenter work with Prometheus?**
Karpenter works with Prometheus in two ways. First, you can optionally connect Karpenter to Prometheus as a metrics source. This lets you configure scaling based on any standard or custom metrics you are already collecting for your cluster. Second, Karpenter uses Prometheus as a metrics store to make scaling decisions.

18. **How does Karpenter compare to the Kubernetes cluster autoscaler?**
The Kubernetes cluster autoscaler reacts to the number of pods running on the cluster and tightly coupled to the type of compute used as well as the number of pods running. This means that cluster autoscaler will not scale a cluster until more pods are running and means that for workloads where you desire to scale on a schedule or over-provision, you need to implement work arounds such as ‘Hollow Pods’ that essentially hack cluster autoscaler. Karpenter can scale directly and proportionally to any metrics that you define. This gives Karpenter the flexibility to implement sophisticated scaling to accommodate a wide variety of use cases such as scheduled scaling or scaling based on external custom metrics.

19.	**Metrics-driven open source autoscaling systems like HPA and KEDA exist today. How is Karpenter different?**
Systems including HPA are similar to Karpenter, but designed to manage pod scaling with the assumption that a reactive node scaling system will ensure enough compute is available for the cluster. Karpenter takes a very similar approach to these exiting metrics-driven systems, and uses many of the same principals, applying them to node scaling.

20.	**Cluster Autoscaler has worked just fine for my clusters, why should I use Karpenter?**
Cluster autoscaler works well for a number of common use cases. However, some use cases such as scheduled scaling or batch processing, you have to do a lot of extra work or accept the performance and resource inefficiencies created by the architecture of Cluster Autoscaler reacting to pending pods. If you have a mix of use cases in your organization, this means that some users have a fundamentally different architecture and approach to scaling their clusters based on what they are doing. Karpenter allows standardization in how scaling works across your entire organization and for all use cases. Its flexibility lets you optimize your cluster scaling to meet any Kubernetes application use case while reducing implementation complexities and differences that cause delays and take significant extra work on behalf of some teams.

21.	**Will the interaction between the autoscaler, metrics server and metrics producer components be always pull based or there are plans to support push architecture, for example, using alert manager alarms from Prometheus?**
The Karpenter architecture will use pulls.

22. **Metrics will be polled periodically to calculate the desired replicas. Is it possible to configure the polling period?**
The default polling period is 10 seconds, though the user can configure this when they setup Karpenter.

23.	**How does Karpenter work for event-driven scaling?**
Karpenter supports event-driven scaling by consuming the events as a metric using a custom metrics producer.

24.	**Is Karpenter open source? How can I contribute?**
Karpenter is an open source project and available on the GitHub. We encourage contributions through GitHub, especially the contribution of new metrics producers for your use case. If you found a bug, have a suggestion, or have something to contribute, please engage with us on GitHub. All engagements must follow our code of conduct.

25.	**Is this just an AWS project?**
This is an AWS initiated project, but we intend it to be used by the entire Kubernetes community. We welcome and encourage anyone to join us! See [contributing](./CONTRIBUTING.md).
