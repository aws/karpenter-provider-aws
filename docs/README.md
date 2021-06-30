<p>Packages:</p>
<ul>
<li>
<a href="#provisioning.karpenter.sh%2fv1alpha2">provisioning.karpenter.sh/v1alpha2</a>
</li>
</ul>
<h2 id="provisioning.karpenter.sh/v1alpha2">provisioning.karpenter.sh/v1alpha2</h2>
<p>
<p>Package v1alpha2 contains API Schema definitions for the v1alpha2 API group</p>
</p>
Resource Types:
<ul></ul>
<h3 id="provisioning.karpenter.sh/v1alpha2.Cluster">Cluster
</h3>
<p>
(<em>Appears on:</em>
<a href="#provisioning.karpenter.sh/v1alpha2.ProvisionerSpec">ProvisionerSpec</a>)
</p>
<p>
<p>Cluster configures the cluster that the provisioner operates against. If
not specified, it will default to using the controller&rsquo;s kube-config.</p>
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
<code>name</code></br>
<em>
string
</em>
</td>
<td>
<p>Name is required to detect implementing cloud provider resources.</p>
</td>
</tr>
<tr>
<td>
<code>caBundle</code></br>
<em>
string
</em>
</td>
<td>
<p>CABundle is required for nodes to verify API Server certificates.</p>
</td>
</tr>
<tr>
<td>
<code>endpoint</code></br>
<em>
string
</em>
</td>
<td>
<p>Endpoint is required for nodes to connect to the API Server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="provisioning.karpenter.sh/v1alpha2.Constraints">Constraints
</h3>
<p>
(<em>Appears on:</em>
<a href="#provisioning.karpenter.sh/v1alpha2.ProvisionerSpec">ProvisionerSpec</a>)
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
<code>taints</code></br>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/#taint-v1-core">
[]Kubernetes core/v1.Taint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Taints will be applied to every node launched by the Provisioner. If
specified, the provisioner will not provision nodes for pods that do not
have matching tolerations.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code></br>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels will be applied to every node launched by the Provisioner unless
overriden by pod node selectors. Well known labels control provisioning
behavior. Additional labels may be supported by your cloudprovider.</p>
</td>
</tr>
<tr>
<td>
<code>zones</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Zones constrains where nodes will be launched by the Provisioner. If
unspecified, defaults to all zones in the region. Cannot be specified if
label &ldquo;topology.kubernetes.io/zone&rdquo; is specified.</p>
</td>
</tr>
<tr>
<td>
<code>instanceTypes</code></br>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>InstanceTypes constrains which instances types will be used for nodes
launched by the Provisioner. If unspecified, it will support all types.
Cannot be specified if label &ldquo;node.kubernetes.io/instance-type&rdquo; is specified.</p>
</td>
</tr>
<tr>
<td>
<code>architecture</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Architecture constrains the underlying node architecture</p>
</td>
</tr>
<tr>
<td>
<code>operatingSystem</code></br>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>OperatingSystem constrains the underlying node operating system</p>
</td>
</tr>
</tbody>
</table>
<h3 id="provisioning.karpenter.sh/v1alpha2.Provisioner">Provisioner
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
<a href="#provisioning.karpenter.sh/v1alpha2.ProvisionerSpec">
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
<code>cluster</code></br>
<em>
<a href="#provisioning.karpenter.sh/v1alpha2.Cluster">
Cluster
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Cluster that launched nodes connect to.</p>
</td>
</tr>
<tr>
<td>
<code>Constraints</code></br>
<em>
<a href="#provisioning.karpenter.sh/v1alpha2.Constraints">
Constraints
</a>
</em>
</td>
<td>
<p>
(Members of <code>Constraints</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Constraints are applied to all nodes launched by this provisioner.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsAfterEmpty</code></br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>TTLSecondsAfterEmpty is the number of seconds the controller will wait
before attempting to terminate a node, measured from when the node is
detected to be empty. A Node is considered to be empty when it does not
have pods scheduled to it, excluding daemonsets.</p>
<p>Termination due to underutilization is disabled if this field is not set.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsUntilExpired</code></br>
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
<code>status</code></br>
<em>
<a href="#provisioning.karpenter.sh/v1alpha2.ProvisionerStatus">
ProvisionerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="provisioning.karpenter.sh/v1alpha2.ProvisionerSpec">ProvisionerSpec
</h3>
<p>
(<em>Appears on:</em>
<a href="#provisioning.karpenter.sh/v1alpha2.Provisioner">Provisioner</a>)
</p>
<p>
<p>ProvisionerSpec is the top level provisioner specification. Provisioners
launch nodes in response to pods where status.conditions[type=unschedulable,
status=true]. Node configuration is driven by through a combination of
provisioner specification (defaults) and pod scheduling constraints
(overrides). A single provisioner is capable of managing highly diverse
capacity within a single cluster and in most cases, only one should be
necessary. For advanced use cases like workload separation and sharding, it&rsquo;s
possible to define multiple provisioners. These provisioners may have
different defaults and can be specifically targeted by pods using
pod.spec.nodeSelector[&ldquo;provisioning.karpenter.sh/name&rdquo;]=$PROVISIONER_NAME.</p>
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
<code>cluster</code></br>
<em>
<a href="#provisioning.karpenter.sh/v1alpha2.Cluster">
Cluster
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Cluster that launched nodes connect to.</p>
</td>
</tr>
<tr>
<td>
<code>Constraints</code></br>
<em>
<a href="#provisioning.karpenter.sh/v1alpha2.Constraints">
Constraints
</a>
</em>
</td>
<td>
<p>
(Members of <code>Constraints</code> are embedded into this type.)
</p>
<em>(Optional)</em>
<p>Constraints are applied to all nodes launched by this provisioner.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsAfterEmpty</code></br>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>TTLSecondsAfterEmpty is the number of seconds the controller will wait
before attempting to terminate a node, measured from when the node is
detected to be empty. A Node is considered to be empty when it does not
have pods scheduled to it, excluding daemonsets.</p>
<p>Termination due to underutilization is disabled if this field is not set.</p>
</td>
</tr>
<tr>
<td>
<code>ttlSecondsUntilExpired</code></br>
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
<h3 id="provisioning.karpenter.sh/v1alpha2.ProvisionerStatus">ProvisionerStatus
</h3>
<p>
(<em>Appears on:</em>
<a href="#provisioning.karpenter.sh/v1alpha2.Provisioner">Provisioner</a>)
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
<code>lastScaleTime</code></br>
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
<code>conditions</code></br>
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
on git commit <code>aa1e430</code>.
</em></p>
