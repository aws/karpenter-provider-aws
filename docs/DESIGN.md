# Metrics Driven Autoscaling

Node Autoscaling (a.k.a. Cluster Autoscaling) is the process of continually adding and removing a cluster’s nodes to meet the resource demands of its pods. As customers scale to increasingly large clusters, autoscaling becomes necessary for both practicality and cost reasons. While overprovisioning is a viable approach at smaller scales, it becomes prohibitively expensive as organizations grow. In response to increasing infrastructure costs, some customers create manual processes to scale node groups, but this approach yields inefficient resource utilization and is error prone. Node autoscaling replaces these manual processes with automation.

## Overview

Metrics driven autoscaling architectures are widespread in the Kubernetes ecosystem, including Kubernetes Horizontal Pod Autoscaler, Knative, and KEDA. Public clouds also use metrics driven architectures, such as [EC2 Autoscaling](https://docs.aws.amazon.com/autoscaling/ec2/userguide/as-scaling-target-tracking.html) and [ECS Autoscaling](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-auto-scaling.html). This approach has been attempted for node autoscaling in kubernetes by project [Cerebral](https://github.com/containership/cerebral), although it is [no longer actively maintained](https://github.com/containership/cerebral/issues/124#issuecomment-679363530). Existing node autoscaling solutions suffer from complexity, inflexibility, performance, and scalability issues. The extensibility of these systems is limited by both configuration mechanism as well as fundamental architectural constraints, preventing users from applying multiple scaling policies to their cluster or using arbitrary signals to control scaling actions.

We will first discuss metrics driven autoscaling in the abstract, before applying the techniques to the domain of node autoscaling. We will also evaluate the landscape of existing Kubernetes ecosystem projects that will be either used to rapidly implement this approach or be aligned with in the long term.

Many aspects of this design contain large subproblems that are beyond the scope of what can be effectively tackled here. The specific component implementations proposed (e.g. metrics producers, autoscaler algorithms) should be viewed as a naive first iteration. The metrics driven architecture for node autoscaling is the basis for future improvements.

## Goals

* Create a flexible and extensible architecture for Node Autoscaling.
* Focus on performance; autoscaling should introduce minimal overhead and not limit cluster size or shape.
* Target the common case with sane defaults; for most use cases “it should just work” with default configuration.
* Provide straightforward tradeoffs for Scalability, Performance, Availability, and Cost.
* Maximize the compatibility with existing solutions within the Kubernetes ecosystem.

## Critical Customer Journeys

* As a customer, I am able to scale up or down on a single or combination of multiple signals.
    * e.g. Capacity Reservations, Scheduled Capacity, Pending Pods, Queue Length
* As a customer, I am able to author my own custom metrics to plug into the autoscaling architecture.
* As a customer, I am able to autoscale multiple node group implementations from different providers.
    * e.g. EC2 Autoscaling Groups, EKS Managed Node Groups, Kops Instance Groups, etc.
* As a customer, I am able to provision and autoscale capacity in the same cluster with disjoint scheduling properties.
    * e.g. GPUs, HPC, Spot Instances, node labels and taints.

## Metrics Driven Autoscaling Architecture

Metrics Driven Autoscaling is broken down into four logical components.

![](./docs/images/overview.jpeg)

These components are able to be implemented flexibly, either combined into a single system (e.g. Kubernetes Cluster Autoscaler), using one system per component (e.g. Horizontal Pod Autoscaler), or some combination (e.g. KEDA, which implements a Metrics Producer/Metrics Server; Knative, which implements a Metrics Producer/Metrics Server/Autoscaler)

### 1. Metrics Producer

Metrics need to come from somewhere. It is the responsibility of a Metrics Producer to provide an accurate, up to date signal to be consumed by the autoscaler. Some metrics producers require explicit configuration, while others are natural to the cluster and are natively available (e.g. CPU utilization/requests). Metrics Producers are pluggable and novel metrics are expected to emerge over time.

### 2. Metrics Server

Once generated, metrics must be stored somewhere. Some metrics server implementations store a long history in a timeseries database (e.g. Prometheus), while others keep a short ephemeral history in memory (e.g. [MetricsServer](https://github.com/kubernetes-sigs/metrics-server)/[KEDA](https://github.com/kedacore/keda)/[Knative](https://github.com/knative/serving)). Often, metrics server implementations implement a pull-based architecture, where the server is responsible for discovering and calling known metrics producers to populate its metrics store. Push based alternatives are also possible, where metrics producers publish their metrics to a metrics server.

### 3. Autoscaler

Metrics values are periodically polled and used to calculate desiredReplicas for the autoscaled resource. The autoscaler contains a generic, black-box autoscaling function that can be parameterized by customers.

replicas = f(currentReplicas, currentMetricValue, desiredMetricValue, params...)

This implementation of the function can be a proportional controller ([see HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details)), a [PID controller](https://en.wikipedia.org/wiki/PID_controller), a [predictive controller](https://netflixtechblog.com/scryer-netflixs-predictive-auto-scaling-engine-part-2-bb9c4f9b9385), or something else entirely. These functions are generic such that customers should be able experiment with different autoscaling functions using the same underlying metrics. Input metrics can be any signal. For example, customers could use a raw signal or transform their metric with some (e.g. step) function before it is input into the autoscaler.

### 4. Replica Controller

The replica controller is responsible for the actual actuation of desiredReplicas. Common examples of this for Kubernetes pods include Deployment, ReplicaSet, and ReplicationController. Kubernetes has the concept of a [scale subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#subresources), which are generically targetable by autoscalers. This component has analogues in Node Group implementations like EC2 Auto Scaling Groups, EKS Managed Node Groups, and GKE Node Pools.

## Node Autoscaling

For node autoscaling, configurations are applied to node groups, drawing parallels to how pod autoscaling applies to deployments. This deviates from existing solutions like the Kubernetes Cluster Autoscaler or Escalator, which globally consider all node groups in their scaling decisions. It gives customers the flexibility to configure policies on different capacity types, but does not limit customers from applying globally computed metrics to a set of node groups.

Assuming that a metrics driven approach results in significantly improved performance, flexibility, and functionality, the following questions emerge.

* What existing systems can be leveraged as part of the metrics pipeline?
* What is the best interface to model autoscaling policies?
* Which autoscaling algorithm(s) should be implemented?
* How can we generically interface with different node group providers?
* What is the set of commonly used metrics producer implementations that should be supported?

While we will strive to leverage existing systems where it makes sense, there will inevitably be component gaps that must be filled in both the short and long term. These components will live under an umbrella project named “Karpenter”.

### Karpenter

Karpenter is a metrics driven node autoscaler. It’s implemented as a Kubernetes controller and defines three custom resources: MetricsProducer, HorizontalAutoscaler, and ScalableNodeGroup. It aligns its interfaces as closely as possible to the Horizontal Pod Autoscaler’s interface, with a long term goal of providing a universal HorizontalAutoscaler definition in upstream Kubernetes. It takes a dependency on [Prometheus](https://prometheus.io/) and provides out-of-the-box support for commonly used metrics producers for Capacity Reservations, Scheduled Capacity, Pending Pods, and Queue Length.

![](./docs/images/design.jpeg)

Before deep diving the design questions, we’ll cover some examples to see how Karpenter works for some common cases.

### Example: Scaling with a Queue

Alice wants to run weekly machine learning jobs using GPUs on her team’s cluster. The resources are expensive, so she wants her workloads to run on capacity that scales up when needed and back down when she’s done. The tasks are enqueued in an AWS SQS Queue, and she has configured a pod [autoscaler](https://keda.sh/docs/1.5/scalers/aws-sqs/) to scale up a pod for each queue message. She needs a node autoscaling solution to provision nodes so that the pods can be scheduled. In the past, she’s used the Kubernetes Cluster Autoscaler, but due to its reactive architecture, [she found that it was too slow](https://github.com/kedacore/keda/issues/637).

She creates three Karpenter resources and applies them with kubectl apply:
```
apiVersion: karpenter.sh/v1alpha1
kind: MetricsProducer
metadata:
  name: ml-training-queue
  namespace: alice
spec:
  queue:
    provider: AWSSQSQueue
    id: arn:aws:sqs:us-west-2:1234567890:alice-ml-training-queue

Her “ml-training-queue” configures Karpenter to periodically monitor queue metrics, such as the length of her AWS SQS Queue. The monitoring process has a Prometheus metrics endpoint at /metrics that returns the a set of metrics about the queue, including queue length. Alice has Prometheus Server installed in her cluster, which dynamically discovers and periodically scrapes the queue length from the metrics producer and stores it in a timeseries database. Alice queries this data manually using karpenter:metrics_producer:queue-length{name="ml-training-queue", namespace="alice"} to make sure that everything is working smoothly.

apiVersion: karpenter.sh/v1alpha1
kind: ScalableNodeGroup
metadata:
  name: ml-training-capacity
spec:
  type: AWSEKSNodeGroup
  id: arn:aws:eks:us-west-2:1234567890:node-group:training-capacity
  replicas: 0 # Will be updated by the HorizontalAutoscaler

---

apiVersion: karpenter.sh/v1alpha1
kind: HorizontalAutoscaler
metadata:
  name: ml-training-capacity-autoscaler
  namespace: alice
spec:
  scaleTargetRef:
    apiVersion: karpenter.sh/v1alpha1
    kind: ScalableNodeGroup
    name: ml-training-capacity
  metrics:
  - type: Prometheus
    prometheus:
      query: karpenter:metrics_producer:queue_length{name="ml-training-queue", namespace="alice"}
      target:
        type: AverageValue
        value: 4
```
Her “ml-training-capacity-autoscaler” defines autoscaling policy for “ml-training-capacity” which manages the replicas in her EKS Node Group. The autoscaler consumes the metric defined by the MetricsProducer resource, karpenter:metrics_producer:queue-length{name="ml-training-queue", namespace="alice"}, and targets an AverageValue of 4 sampled over a default of a 1 minute window. This means that the autoscaler will attempt to maintain a target of 1 node per 4 messages. Once these resources are created, Karpenter will periodically query the metric, compare it to the current number of replicas in the node group, and set the computed value on the ScalableNodeGroup. In response to a watch event from the ScalableNodeGroup’s update, Karpenter’s controller will set the desiredReplicas on the underlying EKS Node Group.

Alice enqueues 2400 tasks into her queue, Karpenter’s PID algorithm quickly converges the node group on a value of 600 desired replicas. At the same time, Alice’s Horizontal Pod Autoscaler responds to the queue length by creating pods. The pods schedule onto the newly created nodes and remove the tasks from the queue. Once the queue is empty, Karpenter’s autoscaling algorithm returns the node group back down to zero.

### Example: Reserving Capacity

Bob is a coworker of Alice and their teams share the same cluster. His team hosts a set of microservices for a product that is gaining new customers every day. Customers choose Bob’s product since it has much lower latency than alternatives. Bob is working with a limited infrastructure budget and needs to minimize costs while making sure his applications are scaling with customer demands. He configures a pod autoscaler for each microservice, which will scale up to maintain low latency as long as capacity is available. Bob’s nodes have 16 cores and 20gb of memory each, and each microservice pod has a resource request of 1 core and 1 gb memory. He is willing to pay for 40% capacity overhead to minimize the chance that a pod will be unschedulable due to unavailable capacity.

He creates two Karpenter resources and applies them with kubectl apply:
```
apiVersion: karpenter.sh/v1alpha1
kind: ScalableNodeGroup
metadata:
  name: bobs-microservices
spec:
  type: AWSEKSNodeGroup
  id: arn:aws:eks:us-west-2:1234567890:node-group:default-capacity

---

apiVersion: karpenter.sh/v1alpha1
kind: HorizontalAutoscaler
metadata:
  name: bobs-microservices-autoscaler
  namespace: bob
spec:
  scaleTargetRef:
    apiVersion: karpenter.sh/v1alpha1
    kind: ScalableNodeGroup
    name: bobs-capacity
  metrics:
  - type: Prometheus
    prometheus:
      query: karpenter:metrics_producer:reserved_capacity{node_group="bobs-microservices", type="cpu"}
      target:
        type: AverageUtilization
        value: 60
  - type: Prometheus
    prometheus:
      query: karpenter:metrics_producer:reserved_capacity{node_group="bobs-microservices", type="memory"}
      target:
        type: AverageUtilization
        value: 60
```

His autoscaler is configured to target two metrics: one for 60% CPU and one for 60% memory. Reservations refers to the resource requests of the pods that are scheduled to the node. Unlike Alice, Bob didn’t need to configure a MetricsProducer. Reservation metrics are cheap to calculate, so Karpenter’s controller automatically produces them for all ScalableNodeGroup resources and exposes them to Prometheus with its /metrics endpoint.

Bob starts out with 9 pods, resulting in (9/16)=.5625 CPU and (9/20)=.45 memory reservations. As his customers’ demand increases, 2 of his microservice pods scale up at the same time. The reservations are now (11/16)=.6875 CPU and 11/20=.55 memory, triggering a scale up. By default, the autoscaler uses largest value in its calculations for scale up and scale down and rounds up to the nearest integer. This conservative approach favors resource availability over cost and Bob can [override](https://godoc.org/k8s.io/api/autoscaling/v2beta2#ScalingPolicySelect) them if necessary.

### Prometheus vs Kubernetes Metrics API

Which metrics technology stack should Karpenter leverage?

Kubernetes has an established landscape for metrics-driven autoscaling. This work was [motivated by and evolved alongside](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/custom-metrics-api.md) the Horizontal Pod Autoscaler. The [Kubernetes monitoring architecture](https://kubernetes.io/docs/tasks/debug-application-cluster/resource-metrics-pipeline/) defines three metrics APIs that are implemented as [kubernetes aggregated APIs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/apiserver-aggregation/).

* metrics.k8s.io provides metrics for pod and node resources like cpu and memory.
* custom.metrics.k8s.io provides metrics for arbitrary kubernetes objects like an Ingress’s qps.
* external.metrics.k8s.io. provides metrics from systems outside of the cluster.

Each API must be implemented by a service in the cluster. Implementations include the [metrics-server](https://github.com/kubernetes-sigs/metrics-server) for metrics.k8s.io, [k8s-prometheus-adapter](https://github.com/DirectXMan12/k8s-prometheus-adapter) for custom.metrics.k8s.io, and [KEDA](https://github.com/kedacore/keda) for external.metrics.k8s.io.

For example, here’s how the Horizontal Pod Autoscaler uses [k8s-prometheus-adapter](https://github.com/DirectXMan12/k8s-prometheus-adapter) and custom.metrics.k8s.io.

![](./docs/images/hpa.jpeg)
Source: https://towardsdatascience.com/kubernetes-hpa-with-custom-metrics-from-prometheus-9ffc201991e

The metrics API is an attractive dependency for several reasons. It uses Kubernetes API semantics, bringing popular Kubernetes features (e.g. kubectl, API standardization) to the domain of metrics. It also enables customers to control access using RBAC, though this isn’t hugely compelling as autoscalers typically operate globally on the cluster and have full permissions to the metrics API (see HPA).

The metrics API has drawbacks. Each API can only have [one implementation](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/monitoring_architecture.md), which creates compatibility challenges when trying to deploy multiple applications that attempt to implement the same metrics API. It’s tempting to make Karpenter an external metrics API implementation by leveraging [existing open source libraries](https://github.com/kubernetes-sigs/custom-metrics-apiserver). In fact, this is exactly how KEDA (a popular metrics driven pod autoscaler) implements its metrics stack. However, this approach would mean that Karpenter could not be deployed to any cluster that uses KEDA or any other external metrics API implementation. This “single-implementation“ design decision has led other autoscaling solutions like [Knative Serving](https://github.com/knative/serving) to avoid dependency on these [metrics APIs](https://github.com/knative/serving/issues/9087#issuecomment-675138178).

Given this constraint, something generic and universal could implement the metrics APIs and then allow systems like Karpenter to feed metrics into it. The [k8s-prometheus-adapter](https://github.com/DirectXMan12/k8s-prometheus-adapter) is a community solution which attempts to be this solution and uses Prometheus as an intermediary, but the adapter must be [manually configured for each metric it exposes](https://github.com/DirectXMan12/k8s-prometheus-adapter/blob/master/docs/config.md). This is a nontrivial customer burden that requires deep knowledge of Prometheus, the k8s-prometheus-adapter, the metrics producer, and Kubernetes metrics APIs. We could explore building a convention for naming external metrics such that the adapter can automatically translate metrics API resources into their external counterparts, removing the need for additional configuration. This [used to be supported](https://github.com/DirectXMan12/k8s-prometheus-adapter#presentation) by k8s-prometheus adapter, but was deprecated in favor of explicit configuration and a configuration generator.

It’s also possible to closely align with KEDA and share an external metrics API server for both pod and node autoscaling. This introduces a project alignment challenge, but it is not insurmountable. Even if this could work, it’s not a perfect solution, as there will continue to be compatibility issues with other Kubernetes metrics API implementations.

Another alternative is to use a metrics solution that doesn’t have the limitations mentioned above; we can build our own ([see Knative](https://github.com/knative/serving/issues/9087#issuecomment-675205480)) or rely on a strong open source candidate.

Prometheus is ubiquitous throughout the Kubernetes ecosystem. It was the second project to graduate from the CNCF after Kubernetes. Both Kubelets and Kubernetes master components expose their metrics through a Prometheus-formatted /metrics endpoint. Controllers built with the Kubebuilder project come with built-in Prometheus metrics support and those that predate Kubebuilder [typically provide integrations](https://istio.io/latest/docs/ops/integrations/prometheus/). There are [well supported client libraries](https://prometheus.io/docs/instrumenting/clientlibs/) that make it straightforward for any process on Kubernetes to become a Prometheus metrics provider. [Grafana](https://grafana.com/), a widely used monitoring dashboard, integrates deeply with Prometheus and can provide visualizations for metrics driven node autoscaling. Another compelling feature of Prometheus is its [discovery API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config), which automatically exposes metrics from providers without user configuration. Finally, it’s worth noting that KEDA has built first class [Prometheus integration](https://keda.sh/docs/1.4/scalers/prometheus/).

There are a few drawbacks to diverging from the existing Kubernetes Metrics APIs. It forces divergence from the Horizontal Pod Autoscaler’s architecture (see next section), which may cause alignment challenges in the future. Kubernetes metrics APIs also come with RBAC support, but Prometheus does not have per-metric authorization. There are also tools like kubectl top which rely on the metrics API, but this command is specific to pod metrics and not useful for metrics used by node autoscaling.

Direct Prometheus integration appears to be the best option. It avoids compatibility issues with other metrics providers. Generic metrics API adapters like k8s-prometheus-adapter create a domain knowledge and configuration burden for customers. This decision has cascading effects to the rest of the design and should be considered very carefully. However, it is a two way door. The API can be flexible to arbitrary metrics stacks, including non-Prometheus alternatives. Prometheus will be considered a soft dependency; it will serve as our reference metrics implementation for Karpenter’s MVP.

### Alignment with the Horizontal Pod Autoscaler API

Is it possible or worthwhile to align with the Horizontal Pod Autoscaler?

The Horizontal Pod Autoscaler (HPA) is a metrics driven pod autoscaling solution in upstream Kubernetes. It’s maintained by SIG Autoscaling and is the canonical solution for the Kubernetes community. Its API has undergone significant changes as Kubernetes has evolved. It initially provided support for scaling a deployment against the average CPU of its Pods, but has since expanded its flexibility in the [v2beta2 API](https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerSpec) to support arbitrary resource targets and custom metrics. It can target any Kubernetes resource that implements the [scale subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#subresources). Today, the existing HPA API is even able to target a Kubernetes resource representing a node group; the only gap is to implement metrics for the domain of node autoscaling.

Unified autoscaling is a powerful concept, as it means that the same implementation can be shared for all autoscaled resources within a cluster. We want to avoid forcing premature alignment, but as long as it doesn’t compromise the design, there is value in keeping these interfaces as similar as possible. Customers need only learn a single architecture for autoscaling, reducing complexity and cognitive load.

There are a couple drawbacks to using the HPA’s API directly. The most obvious is the name, which would be more aptly called HorizontalAutoscaler. Most of its abstractions extend cleanly to Node Groups (e.g. [ScaleTargetRef](https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerSpec), [MetricTarget](https://godoc.org/k8s.io/api/autoscaling/v2beta2#MetricTarget), [ScalingPolicy](https://godoc.org/k8s.io/api/autoscaling/v2beta2#HPAScalingPolicy), [MinReplicas](https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerSpec), [MaxReplicas](https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerSpec), [Behavior](https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerBehavior), StabilizationWindowSeconds (https://godoc.org/k8s.io/api/autoscaling/v2beta2#HPAScalingRules)). Others require slight adjustments (e.g. [ScalingPolicyType](https://godoc.org/k8s.io/api/autoscaling/v2beta2#HPAScalingPolicyType) needs to be tweaked to refer to “replicas” instead of “pods”). However, [MetricSpec](https://godoc.org/k8s.io/api/autoscaling/v2beta2#MetricSpec) is specific to pods and requires changes if relied upon. MetricsSpec has four subfields corresponding to different metrics sources. [ResourceMetricSource](https://godoc.org/k8s.io/api/autoscaling/v2beta2#ResourceMetricSource), which uses the [Resource Metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md) and provides CPU and memory for pods and nodes. [PodsMetricSource](https://godoc.org/k8s.io/api/autoscaling/v2beta2#PodsMetricSource), which is syntactic sugar for [ObjectMetricSource](https://godoc.org/k8s.io/api/autoscaling/v2beta2#ObjectMetricSource), each of which each retrieve metrics from the [Custom Metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/custom-metrics-api.md). [ExternalMetricSource](https://godoc.org/k8s.io/api/autoscaling/v2beta2#ExternalMetricSource), which uses the [External Metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/external-metrics-api.md) to map metric name and namespace to an external object like an AWS SQS Queue.

One approach would be to use the MetricsSpec and its four sources as-is. This requires sourcing all metrics from the Kubernetes metrics APIs (see limitations above). It’s also somewhat awkward, as users would likely never use the PodsMetricSpec or ResourceMetricsSpec to scale their node groups. The primary reason to go this route is alignment with the HorizontalPodAutoscaler and existing Kubernetes metrics APIs. The current Kubernetes metrics architecture is arguably too pod specific and could be changed to be more generic, but we consider engagement with SIG Instrumentation to be out of scope for the short term.

Another option would be use ObjectMetricsSpec and ExternalMetricsSpec and omit pod-specific metrics APIs. This generically covers metrics for both in-cluster and external objects (i.e.  custom.metrics.k8s.io and external.metrics.k8s.io). This approach is cleaner from the perspective of a node autoscaler, but makes future alignment with the HPA more challenging. Pod metrics could still specified, but this removes the syntactic sugar that simplifies the most common use cases for pod autoscaling.

If we choose to integrate with directly with Prometheus metrics (discussed above), there will need to be a new option in the MetricsSpec to specify it as a metrics source (e.g PrometheusMetricSource). Customers would specify a [promql query](https://prometheus.io/docs/prometheus/latest/querying/basics/) to retrieve the metric. The decision to create a PrometheusMetricSource is orthogonal from whether or not we keep existing HPA metrics sources. Either way requires changes to the MetricsSpec; Prometheus support can be built alongside or replace existing metrics sources.

We could also completely diverge from the HPA and start with a minimal autoscaler definition that covers initial node autoscaling use cases. This avoids premature abstraction of a generic autoscaling definition. However, we’re cautious to start from scratch, as it presumes we can design autoscaling APIs better than the HPA. It also makes alignment more challenging in the future.

We will develop a new custom resource that is aligned with the HPA on all fields except for MetricsSpec, which will be tailored specifically to Prometheus. We will vet our theories for metrics driven node autoscaling and revisit upstreaming once the project matures. There are other benefits to owning a custom resource in the short term, like rapid iteration on the API and decoupling the release process from Kubernetes. The decision to directly rely on Prometheus (see previous section) forces divergence from the existing MetricsSpec. If the specs are already diverged, it makes less sense to provide support for metrics sources that aren’t useful for node autoscaling (e.g. ResourceMetricsSpec, PodMetricsSpec). There is still value in using the spec as a starting point, as the concepts are generic to autoscaling and familiar to users.

### Autoscaling Algorithm

Which autoscaling algorithm(s) should be implemented by the horizontal autoscaler?

The HPA implements a [proportional algorithm](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) that scales linearly with the number of replicas. When combined with rounding and windowing logic, this approach is good enough to be used widely by the Kubernetes community. It suffers from a drawback known as [proportional droop](https://www.yld.io/blog/kubernetes-piloting-the-cybernetic-dreamboat/) where the decision fails to take into account the rate of change of the metric, causing it to over or undershoot optimal scale decisions.

A more advanced algorithm called [Proportional Integral Derivative](https://en.wikipedia.org/wiki/PID_controller) (PID) is widely used to solve this problem for thermostats, cruise control, and other control systems. In addition to a proportional term, this approach includes an integral term to capture historical changes and a derivative term to extrapolate from the current rate of change. With well tuned coefficients, this approach can accurately model arbitrary traffic curves for arbitrary use cases.

Predictive autoscaling is an experimental field that leverages machine learning to make scale decisions. This approach is used at [Netflix](https://netflixtechblog.com/scryer-netflixs-predictive-auto-scaling-engine-part-2-bb9c4f9b9385) to learn periodic traffic patterns by analyzing metrics like request per second. Theoretically, deep learning could be used in combination with a rich cluster metrics dataset (e.g. Prometheus) to produce high accuracy black box scale decisions.

The question of which algorithm to use is difficult to answer without deep research and experimentation with real customer workloads. Rather than staking a claim on any particular algorithm, we will leave the door open to iterate and improve options and the default for Karpenter’s autoscaling algorithm. We will initially implement a proportional algorithm.

## APIs

All APIs for Node Autoscaling will be modeled with the [Kubernetes Resource Model](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#kubernetes-objects) (KRM) using [Custom Resource Definitions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) and a long term goal of upstreaming them into the [Kubernetes autoscaling API group](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#api-object). Leveraging the KRM for autoscaling configuration provides myriad benefits including abstraction of interface from implementation, RBAC support, and a familiar user experience with kubectl.

There are three things that need to be modeled:

* a *Metrics Producer* defines a controller that outputs a metric (e.g. Queue Monitor)
* a *Horizontal Autoscaler* associates a scalable resource with a list of target metrics (e.g. HPA)
* a *Scalable Node Group* is able to scale the number of nodes using a “replicas” field

It’s important to note that these APIs should be considered as primitives that can be used to build higher level concepts. For example, this is similar to how KEDA’s [ScaledObject](https://keda.sh/docs/1.4/concepts/scaling-deployments/#scaledobject-spec) resource provides a single abstraction over a HorizontalAutoscaler (HPA) and a MetricsProducer (Queue Length). For this design, description of higher level abstractions for node group autoscaling are considered out of scope. These primitives alone yield a better user experience than existing solutions.

### Metrics Producer

Many metrics (e.g. node capacity utilization) are implicit to the cluster and can made available for autoscaling purposes by Karpenter without explicit user configuration. Other metrics come from other systems and can be plugged directly into the metrics server implementation. However, some metrics require explicit user configuration. Karpenter will provide first-class support for common use cases by generating metrics implicitly where possible and otherwise explicitly configured and generated by a MetricsProducer resource.

We expect this definition to evolve over time (see: Metrics Producer Implementations).
```
apiVersion: karpenter.sh/v1alpha1
kind: MetricsProducer
metadata:
  name: foo-queue-length
spec:
  queue:
    type: AWSSQSQueue
    id: arn:aws:sqs:us-west-2:1234567890:foo-queue
```

### Horizontal Autoscaler

As discussed above, we will align as much as possible with the HPA API. An explicit long term goal for this project is to upstream an abstraction that unifies horizontal autoscaling in Kubernetes. We will mirror the Horizontal Pod Autoscaler API, but will rename references from “pod“ to ”replica“ in API, code comments, and go structs. The only exception to this is the MetricsSpec, which is the only reference to ”pod“ that cannot be generalized to ”replica“.

We expect this definition to evolve over time and intend to eventually upstream it into the core autoscaling API group.
```
apiVersion: karpenter.sh/v1alpha1
kind: HorizontalAutoscaler
metadata:
  name: my-capacity
spec:
  scaleTargetRef:
    apiVersion: karpenter.sh/v1alpha1
    kind: ScalableNodeGroup
    name: my-capacity
  minReplicas: 1
  maxReplicas: 10
  metrics:
  - type: Prometheus
    prometheus:
      query: karpenter:metrics_producer:queue-length{name="foo-queue", namespace="default"}
      target:
        type: AverageValue
        value: 3 # messages per node
```

### Scalable Node Group

The decision to use the HPA’s scaleTargetRef concept creates two requirements for this resource. The API must represent a node group and must implement the [scale subresource](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#subresources). The only responsibility of this resource is to control the replicas field, but this doesn’t preclude targeting a resource that is a full representation of the node group. There currently isn’t a Kubernetes native resource for node group, but there are a number of kubernetes ecosystem projects that model that node groups including [Kops Instance Groups](https://kops.sigs.k8s.io/tutorial/working-with-instancegroups/), [Cluster API Machine Pool](https://github.com/kubernetes-sigs/cluster-api/blob/master/docs/proposals/20190919-machinepool-api.md), [Amazon Controllers for Kubernetes (asg implementation tbd)](https://aws.amazon.com/blogs/containers/aws-controllers-for-kubernetes-ack/), [Anthos Config Management (node pool implementation tbd)](https://cloud.google.com/anthos-config-management/docs/overview), and others.

The Horizontal Autoscaler API is flexible to any and all of these options. However, many Kubernetes customers don’t rely on one of these mechanisms for node group management, instead relying on cloud provider specific abstractions (e.g. ASG). Therefore, we will introduce a resource that can be optionally targeted by HorizontalAutoscaler’s scaleTargetRef for users who don’t have a KRM node group concept. This is a stop gap until other node group representations become more widespread. This object will follow a cloud provider model to provide implementations for different cloud provider solutions such as EKS Managed Node Groups, EC2 Auto Scaling Groups, and others. The only field supported by this resource is replicas, leaving management responsibilities like upgrade up to some other controller.

```
apiVersion: karpenter.sh/v1alpha1
kind: ScalableNodeGroup
metadata:
  name: default-capacity
spec:
  type: AWSEC2AutoScalingGroup
  replicas: 1
```

## Appendix A: Metrics Producer Implementations

Metric driven autoscaling is extensible to as-of-yet unknown use cases. Users will face novel problems and will invent new metrics on which to scale their clusters. As a starting point, we will implement a small set of metrics to support the majority of known use cases. We expect these to evolve significantly with user feedback. As new commonly used metric sources are identified, they will be upstreamed into Karpenter.

### Capacity Reservation

Capacity reservation is perhaps the most straightforward approach to node autoscaling. It’s similar to the commonly used utilization metric for horizontal pod autoscaling. However, instead of scaling on current resource utilization, nodes are scaled on the resource requests (a.k.a. reserved capacity) of existing pods. This approach is used in other container orchestration systems like [AWS ECS](https://aws.amazon.com/blogs/aws/aws-ecs-cluster-auto-scaling-is-now-generally-available/). Note that due to its reliance on current usage of nodes, scale to zero is not supported.

Karpenter can automatically output capacity reservation metrics as they’re cheap to compute. This creates a zero-config starting point for users. As user requirements become more complex, capacity reservations can be used in conjunction with other signals. For example, capacity reservations can be used to drive scale down, while scale up is driven by pending pods. This mimics the Kubernetes Cluster Autoscaler’s algorithm.

Customers will be able to configure this as follows:

### Percentage overprovisioning

```
prometheus:
  query: karpenter:metrics_producer:capacity_reservation{node_group="name", type="cpu"}
  target:
    type: AverageUtilization
    value: 20
```

There are important edge cases with capacity reservations. It’s possible for a node to be unschedulable and for the capacity reservation to not trigger a not scale up. For example, a pod will fail to schedule if it requires 55% of a node’s CPU, all nodes are at 50% reservation, and the reservation is set to 50%. This edge case occurs most frequently when nodes are small, pods are large, and reservations are low. This can be resolved by pairing capacity reservations with other metrics like pending pods, which will ensure that reservations are kept whenever possible, but edge cases will still trigger a scale up.

A similar edge case exists for scale down, where a node could be deleted that holds a pod which is unable to fit in another other node. For further discussion of this case and other scale down edge cases, see Scale Down Edge Cases below.

### Scheduled Capacity

Scheduled capacity is another approach. Many use cases have oscillating traffic patterns such as lower traffic on weekends. Users can used scheduled capacity as a signal to scale up or down by defining a metrics producer with scheduled behavior. Users can layer many crontabs on top of each other to produce a more complex signal.

This signal can be used in combination with other metrics in cases where demand is unexpected. For example, if scheduled capacity recommends that scale down should occur on a Friday afternoon, but due to a traffic spike a capacity reservation disagrees, the replica count will remain high.

```
apiVersion: karpenter.sh/v1alpha1
kind: MetricsProducer
metadata:
  name: foo-queue
spec:
  scheduledCapacity:
    nodeGroup: ...
    behaviors:
    - crontab: ...
      replicas: 4 # Set replicas to 4 at time of cron

prometheus:
  query: karpenter:metrics_producer:scheduled_capacity{node_group="name", type="cpu"}
  target:
    type: AverageValue
    averageValue: 1
```

### Pending Pods

Pending pods operates across multiple node groups. When a pod becomes unschedulable, the algorithm attempts to find a node group which if scaled up, would cause the pod to be scheduled. The MetricsProducer emits a signal per node group that corresponds to whether or not a scale up should occur. The MetricsProducer doesn’t necessarily apply to all node groups in the cluster. This allows some capacity to be scaled using pending pods and others to rely on different metrics producers, which is common for large and diverse clusters.

````
apiVersion: karpenter.sh/v1alpha1
kind: MetricsProducer
metadata:
  name:
spec:
  pendingPods:
    provider: EKSManagedNodeGroups
    nodeGroup: ...

prometheus:
  query: karpenter:metrics_producer:pending_capacity{node_group="name", type="cpu"}
  target:
    type: AverageValue
    value: 1
```

### Queue Length

Queue Length is a metric optimized for event driven and batch computing workloads. Users push messages into a provider like AWS SQS, Apache Kafka, or a large number of alternatives. The length of the queue is periodically monitored by the metrics producer, which is then acted on by the autoscaler. Queue Length is often used for pod autoscaling (see [KEDA](https://github.com/kedacore/keda)), but can also be used to drive node autoscaling. It’s important to distinguish two separate ways this can work, and the tradeoffs for when to use each technique.

The first is to drive pod autoscaling with a queue metric and node autoscaling with pending pods. This is currently how KEDA integrates with the Cluster Autoscaler for node autoscaling. The team has seen some [challenges with this approach](https://github.com/kedacore/keda/issues/637), where reactive autoscaling results in high scheduling latency. Additionally, because the Cluster Autoscaler scales linearly with node groups and unschedulable pods, this approach does not scale to clusters with many thousands of nodes.

The second approach is to drive both pods and nodes using the same queue metric. This can be configured on a 1:1 basis, or as a ratio of n pods per node. The benefit of this approach over pending pods is in both latency and scalability. Pods and nodes are actively scaled up and the decision to scale up a node group is vastly simplified — it’s explicit, rather than implicit.

Queue-based node autoscaling and pending pods are both viable approaches with different trade offs. Pending pods is aligned with kubernetes bin-packing principles, and yields increased capacity utilization for clusters that host diverse workloads and are overprovisioned. In this case, new pods will first schedule into existing capacity before forcing node group scale up via a pending pod autoscaler. However, if users are looking for a simple batch processing workflow of scaleup → do work → scale down, the bin-packing benefits must be weighed against pending pods’ complexity, scalability, and scheduling latency tradeoffs.

```
apiVersion: karpenter.sh/v1alpha1
kind: MetricsProducer
metadata:
  name: foo-queue
spec:
  queue:
    provider: AWSSQSQueue
    id: arn:aws:sqs:us-west-2:1234567890:foo-queue

prometheus:
  query: karpenter:metrics_producer:queueLength{name="foo-queue", namespace="default"}
  target:
    type: AverageValue
    value: 3 # messages per node
```

## Appendix B: Additional Considerations

### Preemptable Nodes

Metrics driven autoscaling supports preemptable node architectures like AWS Spot. It’s incorrect to state that all metrics producers work flawlessly with preemptable nodes, but Karpenter’s flexibility gives customers the ability to apply preemption optimized metrics to preemptable node groups.

Preemptable nodes introduce new requirements; capacity is unavailable more frequently, and it can be be reclaimed by the cloud provider at any time. In autoscaling terms, this results in two cases: failure to scale up and forced scale down. A common solution to mitigate these problems is to rely on multiple preemptable instance types; if one becomes unavailable or removed, the autoscaler can scale up a new instance type that is available. Autoscaling algorithms require that instance types in the same node group are of the same shape (https://aws.github.io/aws-eks-best-practices/cluster-autoscaling/#configuring-your-node-groups) (CPU, memory, etc). This limits the number of instance types that can be used in any given group, increasing the likelihood of insufficient capacity errors. Customers combat this by creating multiple preemptable node groups, each of a different shape.

One way to coordinate scaling across multiple node groups is to let the scheduler drive pod placement and use a capacity reservation metric for each node group. The horizontal autoscalers for each node group are not aware of each other, but they are aware of the pods that the scheduler has assigned to their nodes. As the scheduler adds pods to nodes, the corresponding node group will expand to maintain its reservation. If capacity for any node group becomes unavailable, the node group will fill up until the scheduler is forced to schedule elsewhere. This will gracefully fall back to node groups that have capacity and will continue to scale based off of their reservation metrics.

Alternatively, it’s possible to implement a metrics producer that is purpose built for scaling multiple node groups with respect to the cloud provider’s preemptable node economy. Design of this specific metrics producer is out of scope, but we’ll briefly explore what it might look in order to validate metrics driven autoscaling’s extensibility. This metric would operate on a set of preemptable node groups with deep knowledge of underlying capacity availability and pricing. It would raise or lower the current metric value for each node group, causing the node groups’ horizontal autoscalers to make scaling decisions. It would not interfere with non-preemptable node groups, which would scale on alternative metrics.

### Scale Down Edge Cases

Scale down is a much higher risk operation than scale up. Stateless workloads are typically more tolerant to disruption than stateful or batch computing jobs, but the cost is use case specific. This problem extends beyond autoscaling. For example, nodes must be terminated when system components like the Kubelet are upgraded. They must also be terminated when capacity is migrated from one instance type to another.

Some node autoscalers (e.g. Kubernetes Cluster Autoscaler, Escalator) attempt to deeply understand the pods and nodes they manage in order to intelligently pick which node to delete. This tightly couples the scale down decision to the autoscaler. Metrics driven autoscaling limits itself to controlling only the replicas field of a node group. This means that the scale down decision is delegated to the node group controller. This is critically important to avoid conflict between the controllers. If the autoscaler were to attempt to scale down during a rolling node group upgrade, disagreements can occur causing unintended behavior (https://github.com/aws/containers-roadmap/issues/916).

### Scalability

Each autoscaling component scales differently. However, as the system as a whole reaches scalability bottlenecks, autoscaler responsiveness will be delayed until autoscaling is non-responsive. It’s also possible for components to overwhelm the Kubernetes API server, Prometheus metrics server, or the node group’s underlying API server.

ScalableNodeGroup’s controller leverages the Kubernetes watch API to ensure that only one downstream request is made to the cloud provider for each change in replicas. We don’t anticipate any scalability challenges here.

HorizontalAutoscaler logic varies with the underlying algorithm. Proportional autoscaling decisions are calculated in constant time, while PID autoscaling decisions are polynomial with respect to its time window. Predictive autoscalers can be arbitrarily complex. Across a cluster, the autoscaler scales linearly with the number of node groups that have an autoscaler. If scalability becomes a concern for this component, this can be solved using sharding, as decisions for each node group’s autoscaler is independent.

Prometheus’s scalability is a [large topic](https://improbable.io/blog/thanos-prometheus-at-scale)beyond the scope of Karpenter. We don’t anticipate challenges here, as autoscaling decisions require less history than Prometheus’ other use cases. We will treat Prometheus’ scalability as out of scope.

MetricsProducer logic varies with the underlying algorithm, similar to the HorizontalAutoscaler’s scalability characteristics. Similarly, these processes can be sharded per producer if scalability challenges arise. Some MetricsProducers will break down as the cluster scales, such as PendingCapacity, which requires global analysis of the cluster’s node groups and pending pods. We consider the scalability concerns of any given MetricsProducer out of scope of the metrics driven autoscaling architecture.

### KEDA

There is potential for collaboration between KEDA and Karpenter to share an API and implementation for the MetricsProducer resource for both pod and node autoscaling. The responsibility of this component is identical in both systems, and we could benefit from the velocity gained by sharing metrics implementations. However, KEDA’s current design needs some tweaks to support this.

Firstly, KEDA’s [scaler](https://keda.sh/docs/1.5/scalers/) concept needs to be separated from the [ScaledObject](https://keda.sh/docs/1.4/concepts/scaling-deployments/#scaledobject-spec) resource. Today, ScaledObjects are responsible for defining both MetricsProducer behavior (e.g. queue monitoring) and HorizontalAutoscaler behavior in a single resource. Under the hood, KEDA creates an underlying HorizontalPodAutoscaler resource as part of its implementation. Similarly, if KEDA separated out a MetricsProducer resource, this resource could be be used as part of ScaledObject’s implementation without changes to the ScaledObject API. This factored out MetricsProducer resource could be leveraged by both Karpenter and KEDA. This code could also be shared via libraries, but Karpenter would still need to implement the MetricsProducer custom resource to expose the functionality to users.

There is an additional complication in that KEDA exposes metrics via Kubernetes metrics APIs. If shared, and if Karpenter chooses to rely directly on Prometheus (see above), MetricsProducers will need to expose metrics both as a Prometheus endpoint and to KEDA’s metrics API server. This is backwards compatible and straight forward to implement using [Prometheus client libraries](https://prometheus.io/docs/instrumenting/clientlibs/).

## Appendix C: Future Areas for Exploration

### Autogenerating Horizontal Autoscalers

We recognize that for clusters with many node groups, defining autoscaling configurations will become a burden. We will explore building automated solutions like higher level custom resource abstractions or configuration generators (e.g. [CDK8s](https://github.com/awslabs/cdk8s)).

### Vertical Node Autoscaling / Auto Provisioning

Horizontal Autoscaling limits itself to creating replicas of existing instance types. It’s possible that existing or future workloads could be run more efficiently on new instance types. We will explore systems to modify the instance type of existing node groups (vertical autoscaling) or create and delete node groups (auto provisioning).

## Appendix D: FAQ

### Q: Should MetricsProducer, HorizontalAutoscaler, ScalableNodeGroup be namespaced resources?

Should any or all of our CRDs be namespaced? Namespacing has the benefit of more RBAC control and allows hiding resources from multiple users in the same cluster.

It’s clear that MetricsProducers should be namespaced. Under the hood, they correspond to pods, and should follow similar scoping rules.

Namespacing the HorizontalAutoscaler is nuanced. If we aspire to a global autoscaling definition for Pods and Nodes, this object must be able to be applied to namespaced resources (e.g. deployments) as well as global resources (e.g. ScalableNodeGroups). By this line of reasoning, it would make sense to make the HorizontalAutoscaler a namespaced resource, but it would be somewhat awkward to apply a namespaced Horizontal Autoscaler to a global resource. Future node group representations (ACK/Cluster API) will be namespaced, but it isn’t clear if this will be true for all implementations. It always possible to create a “ClusterHorizontalAutoscaler” that could apply to globally scoped resources.

The Node resource is not namespaced, so it might make sense to do the same for ScalableNodeGroup. Multiple ScalableNodeGroups pointing to the same cloud provider node group will result in undesired behavior. This could still happen if multiple conflicting resources were applied to the same namespace, but this scenario is much less likely. Given that MetricsProducer and HorizontalAutoscaler are both namespaced, it will provide a more intuitive user experience to namespace all three resources.

### Q: Should customers be able to apply multiple HorizontalAutoscaler configurations to the same scaleTargetRef?

The Horizontal Autoscaler API has a []metrics field that lets users pass in multiple metrics. This allows users to specify an OR semantic to scale off of multiple signals. What about multiple Horizontal Autoscaler resources pointing to the same scaleTargetRef? For the HorizontalPodAutoscaler, this results in undesired behavior. For HorizontalAutoscaler, it’s possible to extend the OR semantic across multiple resources. The benefit would be that multiple application developers sharing a node group could scale the node group off of separate policies without being aware of each other.

It’s not clear whether or not this is an intuitive user experience. It’s arguable that this will lead to more confusion and questions of “why did my node group scale unexpectedly?”. We will await customer requests for this feature before considering it further.

### Q: How can we make sure that Karpenter is horizontally scalable?

Karpenter will reconcile resources, automatically produce node group metrics, and it’s likely that these responsibilities will increase over time. There are several paths forward to ensure that the Karpenter controller scales to large clusters. Individual resources are not tightly coupled to each other, so they can be arbitrarily sharded into multiple Karpenter replicas. Initially, metrics producers will be implemented as a goroutine in the Karpenter controller, but can easily be separated out into individual pods.

### Q: Is it possible to abstract away Prometheus in favor of Open Telemetry?

Yes — it may be possible to replace the Prometheus metrics layer with https://opentelemetry.io/ in the future. This project is currently incubating, so we’ll keep a close eye on this project and its roadmap.
