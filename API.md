<p>Packages:</p>
<ul>
<li>
<a href="#karpenter.sh%2fv1alpha3">karpenter.sh/v1alpha3</a>
</li>
</ul>
<h2 id="karpenter.sh/v1alpha3">karpenter.sh/v1alpha3</h2>
<p>
<p>Package v1alpha3 contains API Schema definitions for the v1alpha3 API group</p>
</p>
Resource Types:
<ul></ul>
<h3 id="karpenter.sh/v1alpha3.Constraints">Constraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#karpenter.sh/v1alpha3.ProvisionerSpec">ProvisionerSpec</a>)
</p>
<p>
<p>Constraints are applied to all nodes created by the provisioner. They can be
overriden by NodeSelectors at the pod level.</p>
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
<code>taints</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#taint-v1-core">
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
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels will be applied to every node launched by the Provisioner.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones constrains where nodes will be launched by the Provisioner. If
unspecified, defaults to all zones in the region.</p>
</td>
</tr>
<tr>
<td>
<code>instanceTypes</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>InstanceTypes constrains which instances types will be used for nodes
launched by the Provisioner. If unspecified, defaults to all types.</p>
</td>
</tr>
<tr>
<td>
<code>architectures</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture constrains the underlying node architecture</p>
</td>
</tr>
<tr>
<td>
<code>operatingSystems</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>OperatingSystem constrains the underlying node operating system</p>
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
</tbody>
</table>
<h3 id="karpenter.sh/v1alpha3.Provisioner">Provisioner
</h3>
<p>
<p>Provisioner is the Schema for the Provisioners API</p>
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
<code>metadata</code><br/>
<em>
<a href="https://v1-18.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta">
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
<a href="#karpenter.sh/v1alpha3.ProvisionerSpec">
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
<code>Constraints</code><br/>
<em>
<a href="#karpenter.sh/v1alpha3.Constraints">
Constraints
</a>
</em>
</td>
<td>
<p>
(Members of <code>Constraints</code> are embedded into this type.)
</p>
<p>Constraints are applied to all nodes launched by this provisioner.</p>
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
<p>Termination due to underutilization is disabled if this field is not set.</p>
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
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#karpenter.sh/v1alpha3.ProvisionerStatus">
ProvisionerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.sh/v1alpha3.ProvisionerSpec">ProvisionerSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#karpenter.sh/v1alpha3.Provisioner">Provisioner</a>)
</p>
<p>
<p>ProvisionerSpec is the top level provisioner specification. Provisioners
launch nodes in response to pods where status.conditions[type=unschedulable,
status=true]. Node configuration is driven by through a combination of
provisioner specification (defaults) and pod scheduling constraints
(overrides). A single provisioner is capable of managing highly diverse
capacity within a single cluster and in most cases, only one should be
necessary. It&rsquo;s possible to define multiple provisioners. These provisioners
may have different defaults and can be specifically targeted by pods using
pod.spec.nodeSelector[&ldquo;karpenter.sh/provisioner-name&rdquo;]=$PROVISIONER_NAME.</p>
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
<code>Constraints</code><br/>
<em>
<a href="#karpenter.sh/v1alpha3.Constraints">
Constraints
</a>
</em>
</td>
<td>
<p>
(Members of <code>Constraints</code> are embedded into this type.)
</p>
<p>Constraints are applied to all nodes launched by this provisioner.</p>
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
<p>Termination due to underutilization is disabled if this field is not set.</p>
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
</tbody>
</table>
<h3 id="karpenter.sh/v1alpha3.ProvisionerStatus">ProvisionerStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#karpenter.sh/v1alpha3.Provisioner">Provisioner</a>)
</p>
<p>
<p>ProvisionerStatus defines the observed state of Provisioner</p>
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
<code>lastScaleTime</code><br/>
<em>
knative.dev/pkg/apis.VolatileTime
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
knative.dev/pkg/apis.Conditions
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions is the set of conditions required for this provisioner to scale
its target, and indicates whether or not those conditions are met.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>bf8ae63</code>.
</em></p>
