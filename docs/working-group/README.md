# Working Group
Karpenter's community is open to everyone. All invites are managed through our [Calendar](https://calendar.google.com/calendar/u/0?cid=N3FmZGVvZjVoZWJkZjZpMnJrMmplZzVqYmtAZ3JvdXAuY2FsZW5kYXIuZ29vZ2xlLmNvbQ). Alternatively, you can use our [iCal Export](https://calendar.google.com/calendar/ical/7qfdeof5hebdf6i2rk2jeg5jbk%40group.calendar.google.com/public/basic.ics) to add the events to Outlook or other email providers.


# Notes
Please contribute to our meeting notes by opening a PR.

## Template
1. Community Questions
2. Work Items
3. Demos

# Meeting notes (02/18/21)

## Attendees
- Prateek Gogia
- Elton Pinto
- Viji Sarathy
- Shreyas Srinivasan
- Ellis Tarn
- Nick Tran
- Guy Templeton
- Joseph Burnett
- Elmiko

## Notes
- [ET] Pending pods might not be the only signal for scaling, its reactive and adds latency
    - HPA like metrics scaling with metrics based approach
    - High number of configurations
    - Groupless provisioning for the nodes in the cloud
- [JB] Interesting idea, have you thought about ASG per deployment model, provisioner will create an ASG and focus on deployment and bind a deployment to an ASG
- [Elmiko] Any developer can write a metric producer to feed into Karpenter?
- [ET] Provisioner stuff is still in the design face, and doesn't use Horizontal autoscaler, there is so much configuration and complexity.
- [Elmiko] Are you taking the Karpenter in this new direction with pending pods?
- [ET] We are still talking to customers and figuring out how to approach this.
- [Elmiko] Metrics based approach is more popular with machine learning folks (Data scientists)
- [JB] We could end up in a heterogenous and random collection of nodes, and we lose the groups benefits. Having extra capacity ahead of time.
- [JB] It would be nice to have a way to use the groups.
- [ET] Provisioning group compared to a single provisioner in a cluster
- [JB] Rollout changes can be done incrementally, new pods go to new node, you have enough capacity to rollout but not double the capacity. How do you update a large deployment in flight? Only reason to split things up is if you have some sort of accounting or security use case
- [ET] Responssible for GC, PDB and have some sane policy
- [ET] Provisioning Group has large number of heterogenous nodes.
- [JB] NG to force capacity to a single failure domain.
- [ET] Boils down to online/offline bin packing, check the constraints and group a set of pods that can be run together. Each group is equally deployable. 
- [JB] Defrag the pods comparing the pods on node and calculating the delta within the pods.
- [Elmiko] Sounds interesting, from the prespective of openshift it will com  down to - what resources are customers paying for, debugging(why autoscaler did what it did)? Why did the autoscaler create FOO? How do we avoid cost overrun scenarios.
- [JB] Provisioner is very functional and observabilty can be common
- [ET] Create an audit trail for a bin packing solution and customer can verify why this decision was being and adding observability.
- [JB] Treating it like a blackbox and check what its doing?
- [Elmiko] Openshift prespective, limit to what the provisioner can create, provisioner is not backed by the instances but by the mem/CPU capacity
- [Elmiko] Even if making this shift in direction, it would be nice to still have some metric or signal plugged into the algorithm 
- [JB] Over provisioning knob is going to be important. If you see 10 pods create an extra node for the next pod.
- [ET] Minimize the scheduling latency, to create a right size of synthetic pods 
- [ET] CA add 0-30 seconds, ASG 0-30 seconds and MNG 0-30 seconds based on cluster load size, we removed this machinery and without any optimization saw ~55 seconds for the node to be ready.
- [JB] Having signals is really powerfull, if the provisioner has a over-provision signal. Metrics part is really important for some of the use case.
- [JB] If the metrics are not in your scheduler, you can be a little slow.
- [Elmiko] Using metrics is gateway to be using models later
- [JB] With the provisioner model how will you even add the metrics
- [ET] How does a customer wire a more intelligent metrics
- [JB] Important signal are going to be workload based
- [Elmiko] An API you expose for annotations on the pod/deployment to instruct the provisioner
...

# Meeting notes (02/04/21)

## Attendees:
- Prateek Gogia
- Viji Sarathy
- Subhrangshu Kumar Sarkar
- Shreyas Srinivasan
- Nathan Taber
- Ellis Tarn
- Nick Tran
- Brandon Wagner 
- Guy Templeton

## Notes

- [Ellis] Karpenter received a bunch of customer feedback around pending pods problems
    - Identified CA challenges
        - Zones, mixed instance policies, nodes in node groups are assumed identical
        - Limitations with ASGs with lots of groups
    - New approach, replace ASG with direct create node API
- [Brandon] Supportive of the idea
    - If we can have ASG and create fleet implementation for comparison
- [Ellis] SNG, HA, MP in karpenter are not compatible with this approach
- [Ellis] Focus on reducing pending pods latency to start
- [Guy] Opportunity for this approach, this removes all the guesswork and inaccuracy of CAS, which is quite honestly a pain in the ass.
- [Viji] More basic question, Autoscaling both nodes and pods. Scaling based on a single metric isn't enough. Using a pending pods
    - How Karpenter and CA differs with pending pods approach
    - [Viji] 2-3 minutes to start a new node with CA
    - [Guy] 3 minutes for the nodes to schedulable
    - [Ellis] m5.large took about 63 seconds with ready pods
        - Create fleet is more modern API call with some parameters
    - [Nathan] 
        - CA is slow in case of large clusters
        - We have a requirement for compute resources and need that to be fullfiled by a service.
        - Pre-selected ASG and shape sizes to create the nodes
        - Strip the middle layers that translate the requirements and just ask for what we need.
        - In cases when ASGs are not well pre thought out, CA is limited with the options available, whereas, Karpenter can make these decisions about the shape to select
- [Nathan] ASG wasn't built with the Kubernetes use case and sometimes works well and sometimes doesn't
- [Ellis] Allocator/ De-allocator model, dual controllers constantly provisioning new nodes for more capacity and removing under utilized nodes.
- [Guy] Dedicated node groups, taint ASGs, CA scales those up ASGs, can karpenter do it?
    - [Ellis] When a pod has label and tolerations, we can create specific nodes with those labels and taints
- [Guy] Spot instances - how will that work with this model?
    - We have dedicated node groups for istio gateways, rest is all spot.
- [Guy] CA and de-scheduler don't work nice with each other
- [Ellis] CA has 2 steps of configurations- ASGs and pods
- [Guy] Nicer approach, worry is how flexible that approach is? Seems like a very Google like approach of doing things with auto-provisioner.
    - [Ellis] - Configuring every single pod with a label is a lot of work, compared to having taints at capacity.
- [Ellis] Provisioning and scheduling decisions- 
    - CA emulates scheduling and now karpenter knows instanceID
    - We create a node object in Kubernetes and immediately bind the pod to the node and when the node becomes ready, pod image gets pulled and runs.
    - Kube-scheduler is bypassed in this approach
    - Simulations effort is not used when actual bin-packing is done by Kube-scheduler
    - [Guy] Interesting approach, definetly sold on pod topology routing, can see benefits with bin-packing compared to guessing.
        - You might end up more closely packed compared to the scheduler
    - [Ellis] Scoring doesn't matter anymore because we don't have a set of nodes to select from    
    - [Subhu] How does node ready failure will be handled?
        - Controller has node IDs and constantly compares with cloud provider nodes
    - [Ellis] Bin packing is very cloud provider specific
- [Ellis] Spot termination happens when you get an event, de-allocator can be checking for pricing and improve costs with Spot.
- [Guy] Kops based instances are checking for health and draining nodes. Rebalance Recommendations are already handled by an internal KOPS at Skyscanner

### In scope for Karpenter
- Pid Controller
- Upgrades
- Handle EC2 Instance Failures

# Meeting notes (01/19/21)

## Attendees:
- Ellis Tarn
- Jacob Gabrielson
- Subhrangshu Kumar Sarkar
- Prateek Gogia
- Nick Tran
- Brandon Wagner 
- Guy Templeton

## Notes:
- [Ellis] Conversation with Joe Burnett from sig-autoscaling 
    - HPA should work with scalable node group, as long you use an external metrics.
    - POC is possible working with HPA
- [Ellis] Nick has made good progress in terms of API for scheduled scaling.
    - Design review in upcoming weeks with the community.
- Change the meeting time to Thursday @9AM PT biweekly.

# Meeting Notes (01/12/2021)

## Attendees:
- Ellis Tarn
- Jacob Gabrielson
- Subhrangshu Kumar Sarkar
- Prateek Gogia
- Micah Hausler
- Viji Sarathy
- Shreyas Srinivasan
- Jeremy Cowan
- Guy Templeton

## Discussions
- [Ellis] What are some common use cases for horizontal autoscaling like node auto-scaling?
    - We have 2 metrics producer so far, SQS queue and utilization.
    - Two are in pipeline, cron scheduling and pending pod metrics producers.
- [Jeremy] Have we looked at predictive scaling, analysing metrics overtime and scaling based on history?
    - [Ellis] We are a little far from that, no work started on that yet
- [Viji] How can we pull cloudwatch metrics to Karpenter?
    - [Ellis] We could have a cloud provider model to start with, to add cloudwatch support in horizontal autoscaler
    - Other way would be external metrics API, you get one per cluster, creates problems within the ecosystem.
    - [Viji] CP model pulls the metrics from the cloudwatch APIs and puts in the autoscaler?
        - [Ellis] User would add info in the karpenter spec and an AWS client will try to load the metrics.
        - External metrics API is easy, user has to figure how to configure with cloudwatch API.
        - Universal metrics adapter supporting all the providers and prometheus.
- [Guy] Reg. external metrics API, there is a [proposal](https://github.com/kubernetes-sigs/custom-metrics-apiserver/issues/70) open in the community
    - Custom cloud provider over gRPC [proposal](https://github.com/kubernetes/autoscaler/pull/3140)
- [Guy] Kops did something similar to what Ellis proposed.
- [Subhu] Are we going to support Pod Disruption Budget(PDB) or managed node groups (MNG) equivalent with other providers?
    - [Ellis] karpenter will increase/decrease the number of nodes, someone needs to know which nodes to remove respecting the PDB.
    - CA knows which nodes to scaled down it uses PDB.
    - Node group is the right component deciding which node will not be violating PDB.
- [Guy] Other providers are rellying on PDB in CA for this support. It will be to good discuss with cluster API.
- [Ellis] We might have to support PDB if other providers don't support PDB in node group controllers to maintain neutrality.
- [Viji] Will try to get Karpenter installed and will look into cloudwatch integration.
- [Ellis] Looking to get feedback for installing Karpenter [demo](https://github.com/ellistarn/karpenter-aws-demo)
- [Ellis] Separate sync to discuss pending pods approach in Karpenter
    - [Guy] Space for something less complex as compared to CA, there has been an explosion of flags in CA.

# Meeting Notes (12/4/2020)

## Attendees
@ellistarn
@prateekgogia
@gjtempleton
@shreyas87

## Notes:
-  [Ellis] Shared background
-  [Guy] Cloudwatch metrics, ECS scaling using cloudwatch metrics for autoscaling.
-  [Guy] Karpenter supporting generic cloudwatch metrics?
-  [Guy] Node autoscaling is supported?
-  [Ellis] Cloud provider like model for cloudwatch, provider model exists in scalable node group side.
-  [Ellis] Cloudwatch could support Prometheus API?
-  [Ellis] We can have a direct cloudwatch integration and later refine it?
-  [Guy] Implementing a generic cloud provider in core in CA.
-  [Ellis]  Will explore integration with cloudwatch directly, prefered will be coud provider model.
-  [Guy] Contributions- People in squad will be interested, open to contribute features if it provides value to the team.
-  [Guy] Scaling on non-pending pods and other resources, people have been asking. Karpenter looks promising for these aspects.
-  [Ellis] - Long term goal, upstream project as an alternative. As open as possible and vendor neutral.
-  [Guy] - There is a space for an alternative, given the history CA works around pending pods. Wider adoption possible if mature.
-  [Ellis] - Landing point will be sig-autoscaling.
-  [Guy] - CA lacks cron scheduling scaling.
-  [Ellis] - pending pods are a big requirements.
-  [Prateek] - introduced the pending pods producer proposal.
-  [Ellis] - Move time earlier by an hour and change day to Thursday, create a GH issue to get feedback what time works?