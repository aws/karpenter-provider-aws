---
title: "AWS"
linkTitle: "AWS"
Description: >
  Karpenter AWS API Reference
---
<p>Packages:</p>
<ul>
<li>
<a href="#karpenter.k8s.aws%2fv1alpha1">karpenter.k8s.aws/v1alpha1</a>
</li>
</ul>
<h2 id="karpenter.k8s.aws/v1alpha1">karpenter.k8s.aws/v1alpha1</h2>
<p>Resource Types:</p>
<ul><li>
<a href="#karpenter.k8s.aws/v1alpha1.AWSNodeTemplate">AWSNodeTemplate</a>
</li></ul>
<h3 id="karpenter.k8s.aws/v1alpha1.AWSNodeTemplate">AWSNodeTemplate
</h3>
<div>
<p>AWSNodeTemplate is the Schema for the AWSNodeTemplate API</p>
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
<code>apiVersion</code><br/>
string</td>
<td>
<code>
karpenter.k8s.aws/v1alpha1
</code>
</td>
</tr>
<tr>
<td>
<code>kind</code><br/>
string
</td>
<td><code>AWSNodeTemplate</code></td>
</tr>
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
<a href="#karpenter.k8s.aws/v1alpha1.AWSNodeTemplateSpec">
AWSNodeTemplateSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>userData</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>UserData to be applied to the provisioned nodes.
It must be in the appropriate format based on the AMIFamily in use. Karpenter will merge certain fields into
this UserData to ensure nodes are being provisioned with the correct configuration.</p>
</td>
</tr>
<tr>
<td>
<code>AWS</code><br/>
<em>
<a href="#karpenter.k8s.aws/v1alpha1.AWS">
AWS
</a>
</em>
</td>
<td>
<p>
(Members of <code>AWS</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>amiSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AMISelector discovers AMIs to be used by Amazon EC2 tags.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.k8s.aws/v1alpha1.AWS">AWS
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.k8s.aws/v1alpha1.AWSNodeTemplateSpec">AWSNodeTemplateSpec</a>)
</p>
<div>
<p>AWS contains parameters specific to this cloud provider</p>
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
<code>amiFamily</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AMIFamily is the AMI family that instances use.</p>
</td>
</tr>
<tr>
<td>
<code>context</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Context is a Reserved field in EC2 APIs
<a href="https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html">https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html</a></p>
</td>
</tr>
<tr>
<td>
<code>instanceProfile</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>InstanceProfile is the AWS identity that instances use.</p>
</td>
</tr>
<tr>
<td>
<code>subnetSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SubnetSelector discovers subnets by tags. A value of &ldquo;&rdquo; is a wildcard.</p>
</td>
</tr>
<tr>
<td>
<code>securityGroupSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecurityGroups specify the names of the security groups.</p>
</td>
</tr>
<tr>
<td>
<code>tags</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Tags to be applied on ec2 resources like instances and launch templates.</p>
</td>
</tr>
<tr>
<td>
<code>LaunchTemplate</code><br/>
<em>
<a href="#karpenter.k8s.aws/v1alpha1.LaunchTemplate">
LaunchTemplate
</a>
</em>
</td>
<td>
<p>
(Members of <code>LaunchTemplate</code> are embedded into this type.)
</p>
<p>LaunchTemplate parameters to use when generating an LT</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.k8s.aws/v1alpha1.AWSNodeTemplateSpec">AWSNodeTemplateSpec
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.k8s.aws/v1alpha1.AWSNodeTemplate">AWSNodeTemplate</a>)
</p>
<div>
<p>AWSNodeTemplateSpec is the top level specification for the AWS Karpenter Provider.
This will contain configuration necessary to launch instances in AWS.</p>
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
<code>userData</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>UserData to be applied to the provisioned nodes.
It must be in the appropriate format based on the AMIFamily in use. Karpenter will merge certain fields into
this UserData to ensure nodes are being provisioned with the correct configuration.</p>
</td>
</tr>
<tr>
<td>
<code>AWS</code><br/>
<em>
<a href="#karpenter.k8s.aws/v1alpha1.AWS">
AWS
</a>
</em>
</td>
<td>
<p>
(Members of <code>AWS</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>amiSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AMISelector discovers AMIs to be used by Amazon EC2 tags.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.k8s.aws/v1alpha1.BlockDevice">BlockDevice
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.k8s.aws/v1alpha1.BlockDeviceMapping">BlockDeviceMapping</a>)
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
<code>deleteOnTermination</code><br/>
<em>
bool
</em>
</td>
<td>
<p>DeleteOnTermination indicates whether the EBS volume is deleted on instance termination.</p>
</td>
</tr>
<tr>
<td>
<code>encrypted</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Encrypted indicates whether the EBS volume is encrypted. Encrypted volumes can only
be attached to instances that support Amazon EBS encryption. If you are creating
a volume from a snapshot, you can&rsquo;t specify an encryption value.</p>
</td>
</tr>
<tr>
<td>
<code>iops</code><br/>
<em>
int64
</em>
</td>
<td>
<p>IOPS is the number of I/O operations per second (IOPS). For gp3, io1, and io2 volumes,
this represents the number of IOPS that are provisioned for the volume. For
gp2 volumes, this represents the baseline performance of the volume and the
rate at which the volume accumulates I/O credits for bursting.</p>
<p>The following are the supported values for each volume type:</p>
<ul>
<li><p>gp3: 3,000-16,000 IOPS</p></li>
<li><p>io1: 100-64,000 IOPS</p></li>
<li><p>io2: 100-64,000 IOPS</p></li>
</ul>
<p>For io1 and io2 volumes, we guarantee 64,000 IOPS only for Instances built
on the Nitro System (<a href="https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html#ec2-nitro-instances">https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html#ec2-nitro-instances</a>).
Other instance families guarantee performance up to 32,000 IOPS.</p>
<p>This parameter is supported for io1, io2, and gp3 volumes only. This parameter
is not supported for gp2, st1, sc1, or standard volumes.</p>
</td>
</tr>
<tr>
<td>
<code>kmsKeyID</code><br/>
<em>
string
</em>
</td>
<td>
<p>KMSKeyID (ARN) of the symmetric Key Management Service (KMS) CMK used for encryption.</p>
</td>
</tr>
<tr>
<td>
<code>snapshotID</code><br/>
<em>
string
</em>
</td>
<td>
<p>SnapshotID is the ID of an EBS snapshot</p>
</td>
</tr>
<tr>
<td>
<code>throughput</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Throughput to provision for a gp3 volume, with a maximum of 1,000 MiB/s.
Valid Range: Minimum value of 125. Maximum value of 1000.</p>
</td>
</tr>
<tr>
<td>
<code>volumeSize</code><br/>
<em>
<a href="https://godoc.org/k8s.io/apimachinery/pkg/api/resource#Quantity">
k8s.io/apimachinery/pkg/api/resource.Quantity
</a>
</em>
</td>
<td>
<p>VolumeSize in GiBs. You must specify either a snapshot ID or
a volume size. The following are the supported volumes sizes for each volume
type:</p>
<ul>
<li><p>gp2 and gp3: 1-16,384</p></li>
<li><p>io1 and io2: 4-16,384</p></li>
<li><p>st1 and sc1: 125-16,384</p></li>
<li><p>standard: 1-1,024</p></li>
</ul>
</td>
</tr>
<tr>
<td>
<code>volumeType</code><br/>
<em>
string
</em>
</td>
<td>
<p>VolumeType of the block device.
For more information, see Amazon EBS volume types (<a href="https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html">https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html</a>)
in the Amazon Elastic Compute Cloud User Guide.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.k8s.aws/v1alpha1.BlockDeviceMapping">BlockDeviceMapping
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.k8s.aws/v1alpha1.LaunchTemplate">LaunchTemplate</a>)
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
<code>deviceName</code><br/>
<em>
string
</em>
</td>
<td>
<p>The device name (for example, /dev/sdh or xvdh).</p>
</td>
</tr>
<tr>
<td>
<code>ebs</code><br/>
<em>
<a href="#karpenter.k8s.aws/v1alpha1.BlockDevice">
BlockDevice
</a>
</em>
</td>
<td>
<p>EBS contains parameters used to automatically set up EBS volumes when an instance is launched.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.k8s.aws/v1alpha1.LaunchTemplate">LaunchTemplate
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.k8s.aws/v1alpha1.AWS">AWS</a>)
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
<code>launchTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>LaunchTemplateName for the node. If not specified, a launch template will be generated.
NOTE: This field is for specifying a custom launch template and is exposed in the Spec
as <code>launchTemplate</code> for backwards compatibility.</p>
</td>
</tr>
<tr>
<td>
<code>metadataOptions</code><br/>
<em>
<a href="#karpenter.k8s.aws/v1alpha1.MetadataOptions">
MetadataOptions
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MetadataOptions for the generated launch template of provisioned nodes.</p>
<p>This specifies the exposure of the Instance Metadata Service to
provisioned EC2 nodes. For more information,
see Instance Metadata and User Data
(<a href="https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html">https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html</a>)
in the Amazon Elastic Compute Cloud User Guide.</p>
<p>Refer to recommended, security best practices
(<a href="https://aws.github.io/aws-eks-best-practices/security/docs/iam/#restrict-access-to-the-instance-profile-assigned-to-the-worker-node">https://aws.github.io/aws-eks-best-practices/security/docs/iam/#restrict-access-to-the-instance-profile-assigned-to-the-worker-node</a>)
for limiting exposure of Instance Metadata and User Data to pods.
If omitted, defaults to httpEndpoint enabled, with httpProtocolIPv6
disabled, with httpPutResponseLimit of 2, and with httpTokens
required.</p>
</td>
</tr>
<tr>
<td>
<code>blockDeviceMappings</code><br/>
<em>
<a href="#karpenter.k8s.aws/v1alpha1.BlockDeviceMapping">
[]BlockDeviceMapping
</a>
</em>
</td>
<td>
<p>BlockDeviceMappings to be applied to provisioned nodes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="karpenter.k8s.aws/v1alpha1.MetadataOptions">MetadataOptions
</h3>
<p>
(<em>Appears on: </em><a href="#karpenter.k8s.aws/v1alpha1.LaunchTemplate">LaunchTemplate</a>)
</p>
<div>
<p>MetadataOptions contains parameters for specifying the exposure of the
Instance Metadata Service to provisioned EC2 nodes.</p>
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
<code>httpEndpoint</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTPEndpoint enables or disables the HTTP metadata endpoint on provisioned
nodes. If metadata options is non-nil, but this parameter is not specified,
the default state is &ldquo;enabled&rdquo;.</p>
<p>If you specify a value of &ldquo;disabled&rdquo;, instance metadata will not be accessible
on the node.</p>
</td>
</tr>
<tr>
<td>
<code>httpProtocolIPv6</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTPProtocolIPv6 enables or disables the IPv6 endpoint for the instance metadata
service on provisioned nodes. If metadata options is non-nil, but this parameter
is not specified, the default state is &ldquo;disabled&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>httpPutResponseHopLimit</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTPPutResponseHopLimit is the desired HTTP PUT response hop limit for
instance metadata requests. The larger the number, the further instance
metadata requests can travel. Possible values are integers from 1 to 64.
If metadata options is non-nil, but this parameter is not specified, the
default value is 1.</p>
</td>
</tr>
<tr>
<td>
<code>httpTokens</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>HTTPTokens determines the state of token usage for instance metadata
requests. If metadata options is non-nil, but this parameter is not
specified, the default state is &ldquo;optional&rdquo;.</p>
<p>If the state is optional, one can choose to retrieve instance metadata with
or without a signed token header on the request. If one retrieves the IAM
role credentials without a token, the version 1.0 role credentials are
returned. If one retrieves the IAM role credentials using a valid signed
token, the version 2.0 role credentials are returned.</p>
<p>If the state is &ldquo;required&rdquo;, one must send a signed token header with any
instance metadata retrieval requests. In this state, retrieving the IAM
role credentials always returns the version 2.0 credentials; the version
1.0 credentials are not available.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>ddaf0675</code>.
</em></p>
