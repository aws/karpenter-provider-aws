# API Reference

## Packages
- [karpenter.k8s.aws/v1beta1](#karpenterk8sawsv1beta1)


## karpenter.k8s.aws/v1beta1


### Resource Types
- [EC2NodeClass](#ec2nodeclass)
- [EC2NodeClassList](#ec2nodeclasslist)



#### AMI



AMI contains resolved AMI selector values utilized for node launch

_Appears in:_
- [EC2NodeClassStatus](#ec2nodeclassstatus)

| Field | Description |
| --- | --- |
| `id` _string_ | ID of the AMI |
| `name` _string_ | Name of the AMI |
| `requirements` _[NodeSelectorRequirement](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#nodeselectorrequirement-v1-core) array_ | Requirements of the AMI to be utilized on an instance type |


#### AMISelectorTerm



AMISelectorTerm defines selection logic for an ami used by Karpenter to launch nodes. If multiple fields are used for selection, the requirements are ANDed.

_Appears in:_
- [EC2NodeClassSpec](#ec2nodeclassspec)

| Field | Description |
| --- | --- |
| `tags` _object (keys:string, values:string)_ | Tags is a map of key/value tags used to select subnets Specifying '*' for a value selects all values for a given tag key. |
| `id` _string_ | ID is the ami id in EC2 |
| `name` _string_ | Name is the ami name in EC2. This value is the name field, which is different from the name tag. |
| `owner` _string_ | Owner is the owner for the ami. You can specify a combination of AWS account IDs, "self", "amazon", and "aws-marketplace" |


#### BlockDevice





_Appears in:_
- [BlockDeviceMapping](#blockdevicemapping)

| Field | Description |
| --- | --- |
| `deleteOnTermination` _[bool](#bool)_ | DeleteOnTermination indicates whether the EBS volume is deleted on instance termination. |
| `encrypted` _[bool](#bool)_ | Encrypted indicates whether the EBS volume is encrypted. Encrypted volumes can only be attached to instances that support Amazon EBS encryption. If you are creating a volume from a snapshot, you can't specify an encryption value. |
| `iops` _[int64](#int64)_ | IOPS is the number of I/O operations per second (IOPS). For gp3, io1, and io2 volumes, this represents the number of IOPS that are provisioned for the volume. For gp2 volumes, this represents the baseline performance of the volume and the rate at which the volume accumulates I/O credits for bursting. <br /><br /> The following are the supported values for each volume type: <br /><br /> * gp3: 3,000-16,000 IOPS <br /><br /> * io1: 100-64,000 IOPS <br /><br /> * io2: 100-64,000 IOPS <br /><br /> For io1 and io2 volumes, we guarantee 64,000 IOPS only for Instances built on the Nitro System (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html#ec2-nitro-instances). Other instance families guarantee performance up to 32,000 IOPS. <br /><br /> This parameter is supported for io1, io2, and gp3 volumes only. This parameter is not supported for gp2, st1, sc1, or standard volumes. |
| `kmsKeyID` _string_ | KMSKeyID (ARN) of the symmetric Key Management Service (KMS) CMK used for encryption. |
| `snapshotID` _string_ | SnapshotID is the ID of an EBS snapshot |
| `throughput` _[int64](#int64)_ | Throughput to provision for a gp3 volume, with a maximum of 1,000 MiB/s. Valid Range: Minimum value of 125. Maximum value of 1000. |
| `volumeSize` _[Quantity](#quantity)_ | VolumeSize in `Gi`, `G`, `Ti`, or `T`. You must specify either a snapshot ID or a volume size. The following are the supported volumes sizes for each volume type: <br /><br /> * gp2 and gp3: 1-16,384 <br /><br /> * io1 and io2: 4-16,384 <br /><br /> * st1 and sc1: 125-16,384 <br /><br /> * standard: 1-1,024 |
| `volumeType` _string_ | VolumeType of the block device. For more information, see Amazon EBS volume types (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/EBSVolumeTypes.html) in the Amazon Elastic Compute Cloud User Guide. |


#### BlockDeviceMapping





_Appears in:_
- [EC2NodeClassSpec](#ec2nodeclassspec)

| Field | Description |
| --- | --- |
| `deviceName` _string_ | The device name (for example, /dev/sdh or xvdh). |
| `ebs` _[BlockDevice](#blockdevice)_ | EBS contains parameters used to automatically set up EBS volumes when an instance is launched. |
| `rootVolume` _boolean_ | RootVolume is a flag indicating if this device is mounted as kubelet root dir. You can configure at most one root volume in BlockDeviceMappings. |


#### EC2NodeClass



EC2NodeClass is the Schema for the EC2NodeClass API

_Appears in:_
- [EC2NodeClassList](#ec2nodeclasslist)

| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `karpenter.k8s.aws/v1beta1`
| `kind` _string_ | `EC2NodeClass`
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `spec` _[EC2NodeClassSpec](#ec2nodeclassspec)_ |  |


#### EC2NodeClassList



EC2NodeClassList contains a list of EC2NodeClass



| Field | Description |
| --- | --- |
| `apiVersion` _string_ | `karpenter.k8s.aws/v1beta1`
| `kind` _string_ | `EC2NodeClassList`
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |
| `items` _[EC2NodeClass](#ec2nodeclass) array_ |  |


#### EC2NodeClassSpec



EC2NodeClassSpec is the top level specification for the AWS Karpenter Provider. This will contain configuration necessary to launch instances in AWS.

_Appears in:_
- [EC2NodeClass](#ec2nodeclass)

| Field | Description |
| --- | --- |
| `subnetSelectorTerms` _[SubnetSelectorTerm](#subnetselectorterm) array_ | SubnetSelectorTerms is a list of or subnet selector terms. The terms are ORed. |
| `securityGroupSelectorTerms` _[SecurityGroupSelectorTerm](#securitygroupselectorterm) array_ | SecurityGroupSelectorTerms is a list of or security group selector terms. The terms are ORed. |
| `amiSelectorTerms` _[AMISelectorTerm](#amiselectorterm) array_ | AMISelectorTerms is a list of or ami selector terms. The terms are ORed. |
| `amiFamily` _string_ | AMIFamily is the AMI family that instances use. |
| `userData` _string_ | UserData to be applied to the provisioned nodes. It must be in the appropriate format based on the AMIFamily in use. Karpenter will merge certain fields into this UserData to ensure nodes are being provisioned with the correct configuration. |
| `role` _string_ | Role is the AWS identity that nodes use. This field is immutable. Marking this field as immutable avoids concerns around terminating managed instance profiles from running instances. This field may be made mutable in the future, assuming the correct garbage collection and drift handling is implemented for the old instance profiles on an update. |
| `tags` _object (keys:string, values:string)_ | Tags to be applied on ec2 resources like instances and launch templates. |
| `blockDeviceMappings` _[BlockDeviceMapping](#blockdevicemapping) array_ | BlockDeviceMappings to be applied to provisioned nodes. |
| `detailedMonitoring` _[bool](#bool)_ | DetailedMonitoring controls if detailed monitoring is enabled for instances that are launched |
| `metadataOptions` _[MetadataOptions](#metadataoptions)_ | MetadataOptions for the generated launch template of provisioned nodes. <br /><br /> This specifies the exposure of the Instance Metadata Service to provisioned EC2 nodes. For more information, see Instance Metadata and User Data (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) in the Amazon Elastic Compute Cloud User Guide. <br /><br /> Refer to recommended, security best practices (https://aws.github.io/aws-eks-best-practices/security/docs/iam/#restrict-access-to-the-instance-profile-assigned-to-the-worker-node) for limiting exposure of Instance Metadata and User Data to pods. If omitted, defaults to httpEndpoint enabled, with httpProtocolIPv6 disabled, with httpPutResponseLimit of 2, and with httpTokens required. |
| `context` _string_ | Context is a Reserved field in EC2 APIs https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFleet.html |




#### MetadataOptions



MetadataOptions contains parameters for specifying the exposure of the Instance Metadata Service to provisioned EC2 nodes.

_Appears in:_
- [EC2NodeClassSpec](#ec2nodeclassspec)

| Field | Description |
| --- | --- |
| `httpEndpoint` _string_ | HTTPEndpoint enables or disables the HTTP metadata endpoint on provisioned nodes. If metadata options is non-nil, but this parameter is not specified, the default state is "enabled". <br /><br /> If you specify a value of "disabled", instance metadata will not be accessible on the node. |
| `httpProtocolIPv6` _string_ | HTTPProtocolIPv6 enables or disables the IPv6 endpoint for the instance metadata service on provisioned nodes. If metadata options is non-nil, but this parameter is not specified, the default state is "disabled". |
| `httpPutResponseHopLimit` _[int64](#int64)_ | HTTPPutResponseHopLimit is the desired HTTP PUT response hop limit for instance metadata requests. The larger the number, the further instance metadata requests can travel. Possible values are integers from 1 to 64. If metadata options is non-nil, but this parameter is not specified, the default value is 2. |
| `httpTokens` _string_ | HTTPTokens determines the state of token usage for instance metadata requests. If metadata options is non-nil, but this parameter is not specified, the default state is "required". <br /><br /> If the state is optional, one can choose to retrieve instance metadata with or without a signed token header on the request. If one retrieves the IAM role credentials without a token, the version 1.0 role credentials are returned. If one retrieves the IAM role credentials using a valid signed token, the version 2.0 role credentials are returned. <br /><br /> If the state is "required", one must send a signed token header with any instance metadata retrieval requests. In this state, retrieving the IAM role credentials always returns the version 2.0 credentials; the version 1.0 credentials are not available. |


#### SecurityGroup



SecurityGroup contains resolved SecurityGroup selector values utilized for node launch

_Appears in:_
- [EC2NodeClassStatus](#ec2nodeclassstatus)

| Field | Description |
| --- | --- |
| `id` _string_ | ID of the security group |
| `name` _string_ | Name of the security group |


#### SecurityGroupSelectorTerm



SecurityGroupSelectorTerm defines selection logic for a security group used by Karpenter to launch nodes. If multiple fields are used for selection, the requirements are ANDed.

_Appears in:_
- [EC2NodeClassSpec](#ec2nodeclassspec)

| Field | Description |
| --- | --- |
| `tags` _object (keys:string, values:string)_ | Tags is a map of key/value tags used to select subnets Specifying '*' for a value selects all values for a given tag key. |
| `id` _string_ | ID is the security group id in EC2 |
| `name` _string_ | Name is the security group name in EC2. This value is the name field, which is different from the name tag. |


#### Subnet



Subnet contains resolved Subnet selector values utilized for node launch

_Appears in:_
- [EC2NodeClassStatus](#ec2nodeclassstatus)

| Field | Description |
| --- | --- |
| `id` _string_ | ID of the subnet |
| `zone` _string_ | The associated availability zone |


#### SubnetSelectorTerm



SubnetSelectorTerm defines selection logic for a subnet used by Karpenter to launch nodes. If multiple fields are used for selection, the requirements are ANDed.

_Appears in:_
- [EC2NodeClassSpec](#ec2nodeclassspec)

| Field | Description |
| --- | --- |
| `tags` _object (keys:string, values:string)_ | Tags is a map of key/value tags used to select subnets Specifying '*' for a value selects all values for a given tag key. |
| `id` _string_ | ID is the subnet id in EC2 |


