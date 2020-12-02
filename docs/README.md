<p>Packages:</p>
<ul>
<li>
<a href="#autoscaling.karpenter.sh%2fv1alpha1">autoscaling.karpenter.sh/v1alpha1</a>
</li>
</ul>
<h2 id="autoscaling.karpenter.sh/v1alpha1">autoscaling.karpenter.sh/v1alpha1</h2>
<p>
<p>Package v1alpha1 contains API Schema definitions for the v1alpha1 API group</p>
</p>
Resource Types:
<ul></ul>
<h3 id="autoscaling.karpenter.sh/v1alpha1.Behavior">Behavior
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscalerSpec">HorizontalAutoscalerSpec</a>)
</p>
<p>
<p>Behavior configures the scaling behavior of the target
in both Up and Down directions (scaleUp and scaleDown fields respectively).</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>scaleUp</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalingRules">
ScalingRules
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleUp is scaling policy for scaling Up.
If not set, the default value is the higher of:
* increase no more than 4 replicas per 60 seconds
* double the number of replicas per 60 seconds
No stabilization is used.</p>
</td>
</tr>
<tr>
<td>
<code>scaleDown</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalingRules">
ScalingRules
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScaleDown is scaling policy for scaling Down.
If not set, the default value is to allow to scale down to minReplicas, with a
300 second stabilization window (i.e., the highest recommendation for
the last 300sec is used).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.CrossVersionObjectReference">CrossVersionObjectReference
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscalerSpec">HorizontalAutoscalerSpec</a>)
</p>
<p>
<p>CrossVersionObjectReference contains enough information to let you identify the referred resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kind</code></br>
<em>
string
</em>
</td>
<td>
<p>Kind of the referent; More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds&quot;">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds&rdquo;</a></p>
</td>
</tr>
<tr>
<td>
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name of the referent; More info: <a href="http://kubernetes.io/docs/user-guide/identifiers#names">http://kubernetes.io/docs/user-guide/identifiers#names</a></p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>API version of the referent</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscaler">HorizontalAutoscaler
</h3>
<p>
<p>HorizontalAutoscaler is the Schema for the horizontalautoscalers API</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscalerSpec">
HorizontalAutoscalerSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>scaleTargetRef</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.CrossVersionObjectReference">
CrossVersionObjectReference
</a>
</em>
</td>
<td>
<p>ScaleTargetRef points to the target resource to scale.</p>
</td>
</tr>
<tr>
<td>
<code>minReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>MinReplicas is the lower limit for the number of replicas to which the autoscaler can scale down.
It is allowed to be 0.</p>
</td>
</tr>
<tr>
<td>
<code>maxReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>MaxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.
It cannot be less that minReplicas.</p>
</td>
</tr>
<tr>
<td>
<code>metrics</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.Metric">
[]Metric
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Metrics contains the specifications for which to use to calculate the
desired replica count (the maximum replica count across all metrics will
be used).  The desired replica count is calculated multiplying the
ratio between the target value and the current value by the current
number of replicas.  Ergo, metrics used must decrease as the replica count is
increased, and vice-versa.  See the individual metric source types for
more information about how each type of metric must respond.
If not set, the default metric will be set to 80% average CPU utilization.</p>
</td>
</tr>
<tr>
<td>
<code>behavior</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.Behavior">
Behavior
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Behavior configures the scaling behavior of the target
in both Up and Down directions (scaleUp and scaleDown fields respectively).
If not set, the default ScalingRules for scale up and scale down are used.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscalerStatus">
HorizontalAutoscalerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscalerSpec">HorizontalAutoscalerSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscaler">HorizontalAutoscaler</a>)
</p>
<p>
<p>HorizontalAutoscalerSpec is modeled after <a href="https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerSpec">https://godoc.org/k8s.io/api/autoscaling/v2beta2#HorizontalPodAutoscalerSpec</a>
This enables parity of functionality between Pod and Node autoscaling, with a few minor differences.
1. ObjectSelector is replaced by NodeSelector.
2. Metrics.PodsMetricSelector is replaced by the more generic Metrics.ReplicaMetricSelector.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>scaleTargetRef</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.CrossVersionObjectReference">
CrossVersionObjectReference
</a>
</em>
</td>
<td>
<p>ScaleTargetRef points to the target resource to scale.</p>
</td>
</tr>
<tr>
<td>
<code>minReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>MinReplicas is the lower limit for the number of replicas to which the autoscaler can scale down.
It is allowed to be 0.</p>
</td>
</tr>
<tr>
<td>
<code>maxReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>MaxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.
It cannot be less that minReplicas.</p>
</td>
</tr>
<tr>
<td>
<code>metrics</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.Metric">
[]Metric
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Metrics contains the specifications for which to use to calculate the
desired replica count (the maximum replica count across all metrics will
be used).  The desired replica count is calculated multiplying the
ratio between the target value and the current value by the current
number of replicas.  Ergo, metrics used must decrease as the replica count is
increased, and vice-versa.  See the individual metric source types for
more information about how each type of metric must respond.
If not set, the default metric will be set to 80% average CPU utilization.</p>
</td>
</tr>
<tr>
<td>
<code>behavior</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.Behavior">
Behavior
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Behavior configures the scaling behavior of the target
in both Up and Down directions (scaleUp and scaleDown fields respectively).
If not set, the default ScalingRules for scale up and scale down are used.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscalerStatus">HorizontalAutoscalerStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscaler">HorizontalAutoscaler</a>)
</p>
<p>
<p>HorizontalAutoscalerStatus defines the observed state of HorizontalAutoscaler</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>lastScaleTime</code></br>
<em>
knative.dev/pkg/apis.VolatileTime
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastScaleTime is the last time the HorizontalAutoscaler scaled the number
of pods, used by the autoscaler to control how often the number of pods
is changed.</p>
</td>
</tr>
<tr>
<td>
<code>currentReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>CurrentReplicas is current number of replicas of pods managed by this
autoscaler, as last seen by the autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>desiredReplicas</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>DesiredReplicas is the desired number of replicas of pods managed by this
autoscaler, as last calculated by the autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>currentMetrics</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricStatus">
[]MetricStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CurrentMetrics is the last read state of the metrics used by this
autoscaler.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
knative.dev/pkg/apis.Conditions
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions is the set of conditions required for this autoscaler to scale
its target, and indicates whether or not those conditions are met.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.Metric">Metric
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscalerSpec">HorizontalAutoscalerSpec</a>)
</p>
<p>
<p>Metric is modeled after <a href="https://godoc.org/k8s.io/api/autoscaling/v2beta2#MetricSpec">https://godoc.org/k8s.io/api/autoscaling/v2beta2#MetricSpec</a></p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>prometheus</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.PrometheusMetricSource">
PrometheusMetricSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.MetricStatus">MetricStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.HorizontalAutoscalerStatus">HorizontalAutoscalerStatus</a>)
</p>
<p>
<p>MetricStatus contains status information for the configured metrics source.
This status has a one-of semantic and will only ever contain one value.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>prometheus</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.PrometheusMetricStatus">
PrometheusMetricStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.MetricTarget">MetricTarget
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.PrometheusMetricSource">PrometheusMetricSource</a>)
</p>
<p>
<p>MetricTarget defines the target value, average value, or average utilization of a specific metric</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricTargetType">
MetricTargetType
</a>
</em>
</td>
<td>
<p>Type represents whether the metric type is Utilization, Value, or AverageValue
Value is the target value of the metric (as a quantity).</p>
</td>
</tr>
<tr>
<td>
<code>value</code></br>
<em>
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>averageValue</code></br>
<em>
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<em>(Optional)</em>
<p>AverageValue is the target value of the average of the
metric across all relevant pods (as a quantity)</p>
</td>
</tr>
<tr>
<td>
<code>averageUtilization</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>AverageUtilization is the target value of the average of the
resource metric across all relevant pods, represented as a percentage of
the requested value of the resource for the pods.
Currently only valid for Resource metric source type</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.MetricTargetType">MetricTargetType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricTarget">MetricTarget</a>)
</p>
<p>
<p>MetricTargetType specifies the type of metric being targeted, and should be either &ldquo;Value&rdquo;, &ldquo;AverageValue&rdquo;, or &ldquo;Utilization&rdquo;</p>
</p>
<h3 id="autoscaling.karpenter.sh/v1alpha1.MetricValueStatus">MetricValueStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.PrometheusMetricStatus">PrometheusMetricStatus</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>value</code></br>
<em>
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<em>(Optional)</em>
<p>Value is the current value of the metric (as a quantity).</p>
</td>
</tr>
<tr>
<td>
<code>averageValue</code></br>
<em>
k8s.io/apimachinery/pkg/api/resource.Quantity
</em>
</td>
<td>
<em>(Optional)</em>
<p>AverageValue is the current value of the average of the metric across all
relevant pods (as a quantity)</p>
</td>
</tr>
<tr>
<td>
<code>averageUtilization</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>currentAverageUtilization is the current value of the average of the
resource metric across all relevant pods, represented as a percentage of
the requested value of the resource for the pods.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.MetricsProducer">MetricsProducer
</h3>
<p>
<p>MetricsProducer is the Schema for the MetricsProducers API</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerSpec">
MetricsProducerSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>pendingCapacity</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.PendingCapacitySpec">
PendingCapacitySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingCapacity produces a metric that recommends increases or decreases
to the sizes of a set of node groups based on pending pods.</p>
</td>
</tr>
<tr>
<td>
<code>queue</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.QueueSpec">
QueueSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Queue produces metrics about a specified queue, such as length and age of oldest message,</p>
</td>
</tr>
<tr>
<td>
<code>reservedCapacity</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ReservedCapacitySpec">
ReservedCapacitySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReservedCapacity produces a metric corresponding to the ratio of committed resources
to available resources for the nodes of a specified node group.</p>
</td>
</tr>
<tr>
<td>
<code>scheduledCapacity</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScheduledCapacitySpec">
ScheduledCapacitySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScheduledCapacity produces a metric according to a specified schedule.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerStatus">
MetricsProducerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.MetricsProducerSpec">MetricsProducerSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducer">MetricsProducer</a>)
</p>
<p>
<p>MetricsProducerSpec defines an object that outputs metrics.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pendingCapacity</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.PendingCapacitySpec">
PendingCapacitySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PendingCapacity produces a metric that recommends increases or decreases
to the sizes of a set of node groups based on pending pods.</p>
</td>
</tr>
<tr>
<td>
<code>queue</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.QueueSpec">
QueueSpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Queue produces metrics about a specified queue, such as length and age of oldest message,</p>
</td>
</tr>
<tr>
<td>
<code>reservedCapacity</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ReservedCapacitySpec">
ReservedCapacitySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ReservedCapacity produces a metric corresponding to the ratio of committed resources
to available resources for the nodes of a specified node group.</p>
</td>
</tr>
<tr>
<td>
<code>scheduledCapacity</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScheduledCapacitySpec">
ScheduledCapacitySpec
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScheduledCapacity produces a metric according to a specified schedule.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.MetricsProducerStatus">MetricsProducerStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducer">MetricsProducer</a>)
</p>
<p>
<p>MetricsProducerStatus defines the observed state of the resource.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pendingCapacity</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.PendingCapacityStatus">
PendingCapacityStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>queue</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.QueueStatus">
QueueStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>reservedCapacity</code></br>
<em>
map[k8s.io/api/core/v1.ResourceName]string
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>scheduledCapacity</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScheduledCapacityStatus">
ScheduledCapacityStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
knative.dev/pkg/apis.Conditions
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions is the set of conditions required for the metrics producer to
successfully publish metrics to the metrics server</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.NodeGroupType">NodeGroupType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroupSpec">ScalableNodeGroupSpec</a>)
</p>
<p>
<p>NodeGroupType refers to the implementation of the ScalableNodeGroup</p>
</p>
<h3 id="autoscaling.karpenter.sh/v1alpha1.PendingCapacitySpec">PendingCapacitySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerSpec">MetricsProducerSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>nodeSelector</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>NodeSelector specifies a node group. The selector must uniquely identify a set of nodes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.PendingCapacityStatus">PendingCapacityStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerStatus">MetricsProducerStatus</a>)
</p>
<p>
</p>
<h3 id="autoscaling.karpenter.sh/v1alpha1.PendingPodsSpec">PendingPodsSpec
</h3>
<p>
<p>PendingPodsSpec outputs a metric that identifies scheduling opportunities for pending pods in specified node groups.
If multiple pending pods metrics producers exist, the algorithm will ensure that only a single node group scales up.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>nodeSelector</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>NodeSelector specifies a node group. Each selector must uniquely identify a set of nodes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.PrometheusMetricSource">PrometheusMetricSource
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.Metric">Metric</a>)
</p>
<p>
<p>PrometheusMetricSource defines a metric in Prometheus</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>query</code></br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>target</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricTarget">
MetricTarget
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.PrometheusMetricStatus">PrometheusMetricStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricStatus">MetricStatus</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>query</code></br>
<em>
string
</em>
</td>
<td>
<p>Query of the metric</p>
</td>
</tr>
<tr>
<td>
<code>current</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricValueStatus">
MetricValueStatus
</a>
</em>
</td>
<td>
<p>Current contains the current value for the given metric</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.QueueSpec">QueueSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerSpec">MetricsProducerSpec</a>)
</p>
<p>
<p>QueueSpec outputs metrics for a queue.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.QueueType">
QueueType
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.QueueStatus">QueueStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerStatus">MetricsProducerStatus</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>length</code></br>
<em>
int64
</em>
</td>
<td>
<p>Length of the Queue</p>
</td>
</tr>
<tr>
<td>
<code>oldestMessageAgeSeconds</code></br>
<em>
int64
</em>
</td>
<td>
<p>The age of the oldest message in the queue in seconds</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.QueueType">QueueType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.QueueSpec">QueueSpec</a>)
</p>
<p>
<p>QueueType corresponds to an implementation of a queue</p>
</p>
<h3 id="autoscaling.karpenter.sh/v1alpha1.QueueValidator">QueueValidator
</h3>
<p>
</p>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ReservedCapacitySpec">ReservedCapacitySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerSpec">MetricsProducerSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>nodeSelector</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>NodeSelector specifies a node group. The selector must uniquely identify a set of nodes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroup">ScalableNodeGroup
</h3>
<p>
<p>ScalableNodeGroup is the Schema for the ScalableNodeGroups API</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroupSpec">
ScalableNodeGroupSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>Replicas is the desired number of replicas for the targeted Node Group</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.NodeGroupType">
NodeGroupType
</a>
</em>
</td>
<td>
<p>Type for the resource of name ScalableNodeGroup.ObjectMeta.Name</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID to identify the underlying resource</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroupStatus">
ScalableNodeGroupStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroupSpec">ScalableNodeGroupSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroup">ScalableNodeGroup</a>)
</p>
<p>
<p>ScalableNodeGroupSpec is an abstract representation for a Cloud Provider&rsquo;s Node Group. It implements
<a href="https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#scale-subresource">https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#scale-subresource</a>
which enables it to be targeted by Horizontal Pod Autoscalers.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<p>Replicas is the desired number of replicas for the targeted Node Group</p>
</td>
</tr>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.NodeGroupType">
NodeGroupType
</a>
</em>
</td>
<td>
<p>Type for the resource of name ScalableNodeGroup.ObjectMeta.Name</p>
</td>
</tr>
<tr>
<td>
<code>id</code></br>
<em>
string
</em>
</td>
<td>
<p>ID to identify the underlying resource</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroupStatus">ScalableNodeGroupStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroup">ScalableNodeGroup</a>)
</p>
<p>
<p>ScalableNodeGroupStatus holds status information for the ScalableNodeGroup</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Replicas displays the actual size of the ScalableNodeGroup
at the time of the last reconciliation</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code></br>
<em>
knative.dev/pkg/apis.Conditions
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions is the set of conditions required for the scalable node group
to successfully enforce the replica count of the underlying group</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScalableNodeGroupValidator">ScalableNodeGroupValidator
</h3>
<p>
</p>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScalingPolicy">ScalingPolicy
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalingRules">ScalingRules</a>)
</p>
<p>
<p>ScalingPolicy is a single policy which must hold true for a specified past interval.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalingPolicyType">
ScalingPolicyType
</a>
</em>
</td>
<td>
<p>Type is used to specify the scaling policy.</p>
</td>
</tr>
<tr>
<td>
<code>value</code></br>
<em>
int32
</em>
</td>
<td>
<p>Value contains the amount of change which is permitted by the policy.
It must be greater than zero</p>
</td>
</tr>
<tr>
<td>
<code>periodSeconds</code></br>
<em>
int32
</em>
</td>
<td>
<p>PeriodSeconds specifies the window of time for which the policy should hold true.
PeriodSeconds must be greater than zero and less than or equal to 1800 (30 min).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScalingPolicySelect">ScalingPolicySelect
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalingRules">ScalingRules</a>)
</p>
<p>
<p>ScalingPolicySelect is used to specify which policy should be used while scaling in a certain direction</p>
</p>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScalingPolicyType">ScalingPolicyType
(<code>string</code> alias)</p></h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalingPolicy">ScalingPolicy</a>)
</p>
<p>
<p>ScalingPolicyType is the type of the policy which could be used while making scaling decisions.</p>
</p>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScalingRules">ScalingRules
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.Behavior">Behavior</a>)
</p>
<p>
<p>ScalingRules configures the scaling behavior for one direction.
These Rules are applied after calculating DesiredReplicas from metrics for the HPA.
They can limit the scaling velocity by specifying scaling policies.
They can prevent flapping by specifying the stabilization window, so that the
number of replicas is not set instantly, instead, the safest value from the stabilization
window is chosen.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>stabilizationWindowSeconds</code></br>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>StabilizationWindowSeconds is the number of seconds for which past recommendations should be
considered while scaling up or scaling down.
StabilizationWindowSeconds must be greater than or equal to zero and less than or equal to 3600 (one hour).
If not set, use the default values:
- For scale up: 0 (i.e. no stabilization is done).
- For scale down: 300 (i.e. the stabilization window is 300 seconds long).</p>
</td>
</tr>
<tr>
<td>
<code>selectPolicy</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalingPolicySelect">
ScalingPolicySelect
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>selectPolicy is used to specify which policy should be used.
If not set, the default value MaxPolicySelect is used.</p>
</td>
</tr>
<tr>
<td>
<code>policies</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScalingPolicy">
[]ScalingPolicy
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>policies is a list of potential scaling polices which can be used during scaling.
At least one policy must be specified, otherwise the ScalingRules will be discarded as invalid</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScheduledBehavior">ScheduledBehavior
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScheduledCapacitySpec">ScheduledCapacitySpec</a>)
</p>
<p>
<p>ScheduledBehavior defines a crontab which sets the metric to a specific replica value on a schedule.</p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>crontab</code></br>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>replicas</code></br>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScheduledCapacitySpec">ScheduledCapacitySpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerSpec">MetricsProducerSpec</a>)
</p>
<p>
</p>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>nodeSelector</code></br>
<em>
map[string]string
</em>
</td>
<td>
<p>NodeSelector specifies a node group. The selector must uniquely identify a set of nodes.</p>
</td>
</tr>
<tr>
<td>
<code>behaviors</code></br>
<em>
<a href="#autoscaling.karpenter.sh/v1alpha1.ScheduledBehavior">
[]ScheduledBehavior
</a>
</em>
</td>
<td>
<p>Behaviors may be layered to achieve complex scheduling autoscaling logic</p>
</td>
</tr>
</tbody>
</table>
<h3 id="autoscaling.karpenter.sh/v1alpha1.ScheduledCapacityStatus">ScheduledCapacityStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#autoscaling.karpenter.sh/v1alpha1.MetricsProducerStatus">MetricsProducerStatus</a>)
</p>
<p>
</p>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>52b7290</code>.
</em></p>
