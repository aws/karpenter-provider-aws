---
title: "Core"
linkTitle: "Core"
Description: >
  Karpenter Core API Reference
---
<p>Packages:</p>
<ul>
<li>
<a href="#karpenter.sh%2fv1alpha5">karpenter.sh/v1alpha5</a>
</li>
</ul>
<h2 id="karpenter.sh/v1alpha5">karpenter.sh/v1alpha5</h2>
<p>Resource Types:</p>
<ul></ul>
<h3 id="karpenter.sh/v1alpha5.Consolidation">Consolidation
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.sh/v1alpha5.ProvisionerSpec">ProvisionerSpec</a>)
</p>
<div>
</div>
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
<code>enabled</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Enabled enables consolidation if it has been set</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.sh/v1alpha5.KubeletConfiguration">KubeletConfiguration
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.sh/v1alpha5.ProvisionerSpec">ProvisionerSpec</a>)
</p>
<div>
<p>KubeletConfiguration defines args to be used when configuring kubelet on provisioned nodes.
They are a subset of the upstream types, recognizing not all options may be supported.
Wherever possible, the types and names should reflect the upstream kubelet types.
<a href="https://pkg.go.dev/k8s.io/kubelet/config/v1beta1#KubeletConfiguration">https://pkg.go.dev/k8s.io/kubelet/config/v1beta1#KubeletConfiguration</a>
<a href="https://github.com/kubernetes/kubernetes/blob/9f82d81e55cafdedab619ea25cabf5d42736dacf/cmd/kubelet/app/options/options.go#L53">https://github.com/kubernetes/kubernetes/blob/9f82d81e55cafdedab619ea25cabf5d42736dacf/cmd/kubelet/app/options/options.go#L53</a></p>
</div>
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
<code>clusterDNS</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>clusterDNS is a list of IP addresses for the cluster DNS server.
Note that not all providers may use all addresses.</p>
</td>
</tr>
<tr>
<td>
<code>containerRuntime</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ContainerRuntime is the container runtime to be used with your worker nodes.</p>
</td>
</tr>
<tr>
<td>
<code>maxPods</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MaxPods is an override for the maximum number of pods that can run on
a worker node instance.</p>
</td>
</tr>
<tr>
<td>
<code>podsPerCore</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodsPerCore is an override for the number of pods that can run on a worker node
instance based on the number of cpu cores. This value cannot exceed MaxPods, so, if
MaxPods is a lower value, that value will be used.</p>
</td>
</tr>
<tr>
<td>
<code>systemReserved</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SystemReserved contains resources reserved for OS system daemons and kernel memory.</p>
</td>
</tr>
<tr>
<td>
<code>kubeReserved</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeReserved contains resources reserved for Kubernetes system components.</p>
</td>
</tr>
<tr>
<td>
<code>evictionHard</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionHard is the map of signal names to quantities that define hard eviction thresholds</p>
</td>
</tr>
<tr>
<td>
<code>evictionSoft</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionSoft is the map of signal names to quantities that define soft eviction thresholds</p>
</td>
</tr>
<tr>
<td>
<code>evictionSoftGracePeriod</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#Duration">
map[string]k8s.io/apimachinery/pkg/apis/meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionSoftGracePeriod is the map of signal names to quantities that define grace periods for each eviction signal</p>
</td>
</tr>
<tr>
<td>
<code>evictionMaxPodGracePeriod</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>EvictionMaxPodGracePeriod is the maximum allowed grace period (in seconds) to use when terminating pods in
response to soft eviction thresholds being met.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.sh/v1alpha5.Limits">Limits
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.sh/v1alpha5.ProvisionerSpec">ProvisionerSpec</a>)
</p>
<div>
<p>Limits define bounds on the resources being provisioned by Karpenter</p>
</div>
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
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<p>Resources contains all the allocatable resources that Karpenter supports for limiting.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.sh/v1alpha5.ProviderRef">ProviderRef
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.sh/v1alpha5.ProvisionerSpec">ProvisionerSpec</a>)
</p>
<div>
</div>
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
<code>kind</code><br/>
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
<code>name</code><br/>
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
<code>apiVersion</code><br/>
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
<h3 id="karpenter.sh/v1alpha5.Provisioner">Provisioner
</h3>
<div>
<p>Provisioner is the Schema for the Provisioners API</p>
</div>
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
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta">
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
<code>spec</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.ProvisionerSpec">
ProvisionerSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Annotations are applied to every node.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels are layered with Requirements and applied to every node.</p>
</td>
</tr>
<tr>
<td>
<code>taints</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#taint-v1-core">
[]Kubernetes core/v1.Taint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Taints will be applied to every node launched by the Provisioner. If
specified, the provisioner will not provision nodes for pods that do not
have matching tolerations. Additional taints will be created that match
pod tolerations on a per-node basis.</p>
</td>
</tr>
<tr>
<td>
<code>startupTaints</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#taint-v1-core">
[]Kubernetes core/v1.Taint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartupTaints are taints that are applied to nodes upon startup which are expected to be removed automatically
within a short period of time, typically by a DaemonSet that tolerates the taint. These are commonly used by
daemonsets to allow initialization and enforce startup ordering.  StartupTaints are ignored for provisioning
purposes in that pods are not required to tolerate a StartupTaint in order to have nodes provisioned for them.</p>
</td>
</tr>
<tr>
<td>
<code>requirements</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#nodeselectorrequirement-v1-core">
[]Kubernetes core/v1.NodeSelectorRequirement
</a>
</em>
</td>
<td>
<p>Requirements are layered with Labels and applied to every node.</p>
</td>
</tr>
<tr>
<td>
<code>kubeletConfiguration</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.KubeletConfiguration">
KubeletConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeletConfiguration are options passed to the kubelet when provisioning nodes</p>
</td>
</tr>
<tr>
<td>
<code>provider</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<p>Provider contains fields specific to your cloudprovider.</p>
</td>
</tr>
<tr>
<td>
<code>providerRef</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.ProviderRef">
ProviderRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderRef is a reference to a dedicated CRD for the chosen provider, that holds
additional configuration options</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsAfterEmpty</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>TTLSecondsAfterEmpty is the number of seconds the controller will wait
before attempting to delete a node, measured from when the node is
detected to be empty. A Node is considered to be empty when it does not
have pods scheduled to it, excluding daemonsets.</p>
<p>Termination due to no utilization is disabled if this field is not set.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsUntilExpired</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>TTLSecondsUntilExpired is the number of seconds the controller will wait
before terminating a node, measured from when the node is created. This
is useful to implement features like eventually consistent node upgrade,
memory leak protection, and disruption testing.</p>
<p>Termination due to expiration is disabled if this field is not set.</p>
</td>
</tr>
<tr>
<td>
<code>limits</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.Limits">
Limits
</a>
</em>
</td>
<td>
<p>Limits define a set of bounds for provisioning capacity.</p>
</td>
</tr>
<tr>
<td>
<code>weight</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Weight is the priority given to the provisioner during scheduling. A higher
numerical weight indicates that this provisioner will be ordered
ahead of other provisioners with lower weights. A provisioner with no weight
will be treated as if it is a provisioner with a weight of 0.</p>
</td>
</tr>
<tr>
<td>
<code>consolidation</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.Consolidation">
Consolidation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Consolidation are the consolidation parameters</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.ProvisionerStatus">
ProvisionerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.sh/v1alpha5.ProvisionerSpec">ProvisionerSpec
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.sh/v1alpha5.Provisioner">Provisioner</a>)
</p>
<div>
<p>ProvisionerSpec is the top level provisioner specification. Provisioners
launch nodes in response to pods that are unschedulable. A single provisioner
is capable of managing a diverse set of nodes. Node properties are determined
from a combination of provisioner and pod scheduling constraints.</p>
</div>
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
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Annotations are applied to every node.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels are layered with Requirements and applied to every node.</p>
</td>
</tr>
<tr>
<td>
<code>taints</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#taint-v1-core">
[]Kubernetes core/v1.Taint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Taints will be applied to every node launched by the Provisioner. If
specified, the provisioner will not provision nodes for pods that do not
have matching tolerations. Additional taints will be created that match
pod tolerations on a per-node basis.</p>
</td>
</tr>
<tr>
<td>
<code>startupTaints</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#taint-v1-core">
[]Kubernetes core/v1.Taint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartupTaints are taints that are applied to nodes upon startup which are expected to be removed automatically
within a short period of time, typically by a DaemonSet that tolerates the taint. These are commonly used by
daemonsets to allow initialization and enforce startup ordering.  StartupTaints are ignored for provisioning
purposes in that pods are not required to tolerate a StartupTaint in order to have nodes provisioned for them.</p>
</td>
</tr>
<tr>
<td>
<code>requirements</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#nodeselectorrequirement-v1-core">
[]Kubernetes core/v1.NodeSelectorRequirement
</a>
</em>
</td>
<td>
<p>Requirements are layered with Labels and applied to every node.</p>
</td>
</tr>
<tr>
<td>
<code>kubeletConfiguration</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.KubeletConfiguration">
KubeletConfiguration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubeletConfiguration are options passed to the kubelet when provisioning nodes</p>
</td>
</tr>
<tr>
<td>
<code>provider</code><br/>
<em>
k8s.io/apimachinery/pkg/runtime.RawExtension
</em>
</td>
<td>
<p>Provider contains fields specific to your cloudprovider.</p>
</td>
</tr>
<tr>
<td>
<code>providerRef</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.ProviderRef">
ProviderRef
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderRef is a reference to a dedicated CRD for the chosen provider, that holds
additional configuration options</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsAfterEmpty</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>TTLSecondsAfterEmpty is the number of seconds the controller will wait
before attempting to delete a node, measured from when the node is
detected to be empty. A Node is considered to be empty when it does not
have pods scheduled to it, excluding daemonsets.</p>
<p>Termination due to no utilization is disabled if this field is not set.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsUntilExpired</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>TTLSecondsUntilExpired is the number of seconds the controller will wait
before terminating a node, measured from when the node is created. This
is useful to implement features like eventually consistent node upgrade,
memory leak protection, and disruption testing.</p>
<p>Termination due to expiration is disabled if this field is not set.</p>
</td>
</tr>
<tr>
<td>
<code>limits</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.Limits">
Limits
</a>
</em>
</td>
<td>
<p>Limits define a set of bounds for provisioning capacity.</p>
</td>
</tr>
<tr>
<td>
<code>weight</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Weight is the priority given to the provisioner during scheduling. A higher
numerical weight indicates that this provisioner will be ordered
ahead of other provisioners with lower weights. A provisioner with no weight
will be treated as if it is a provisioner with a weight of 0.</p>
</td>
</tr>
<tr>
<td>
<code>consolidation</code><br/>
<em>
<a href="#karpenter.sh/v1alpha5.Consolidation">
Consolidation
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Consolidation are the consolidation parameters</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.sh/v1alpha5.ProvisionerStatus">ProvisionerStatus
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.sh/v1alpha5.Provisioner">Provisioner</a>)
</p>
<div>
<p>ProvisionerStatus defines the observed state of Provisioner</p>
</div>
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
<code>lastScaleTime</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/#VolatileTime">
Knative VolatileTime
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastScaleTime is the last time the Provisioner scaled the number
of nodes</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="https://pkg.go.dev/knative.dev/pkg/apis/#Conditions">
Knative Conditions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions is the set of conditions required for this provisioner to scale
its target, and indicates whether or not those conditions are met.</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcelist-v1-core">
Kubernetes core/v1.ResourceList
</a>
</em>
</td>
<td>
<p>Resources is the list of resources that have been provisioned.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>ddaf0675</code>.
</em></p>
