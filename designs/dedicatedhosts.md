# Dedicated host support
## Background

As described in [3182](https://github.com/aws/karpenter/issues/3182) AWS provides the 
ability to launch EC2 instances on [Dedicated Host hardware](https://docs.aws.amazon.com/license-manager/latest/userguide/host-resource-groups.html)
Dedicated hosts are often used to pay for ec2 usage as capital expenditure (CapEx)
rather than an operating expense (OpEx). This is often useful for publicly listed
companies that want to manage their revenue to OpEx ratio.

AWS allows the allocation of ec2 instances to Dedidcated Host Host Resource Groups (HRG)
through the use of launchtemplates. 
Detailed documentation for launchtemplates and host resource groups can be found here:
- https://docs.aws.amazon.com/autoscaling/ec2/userguide/create-launch-template.html#advanced-settings-for-your-launch-template
- https://docs.aws.amazon.com/license-manager/latest/userguide/host-resource-groups.html

## Solutions

aws-karpenter already supports a set of [LaunchTemplate configuration](https://github.com/aws/karpenter/blob/main/pkg/apis/v1alpha1/awsnodetemplate.go)
with design document: https://github.com/aws/karpenter/blob/main/designs/aws-launch-templates-v2.md

1. Implement the minimal fields from https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-ec2-launchtemplate-launchtemplatedata-placement.html
specifically `placement.hostResourceGroupArn` and `licenseConfiguration`

this would look like:

```
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  licenseConfiguration: arn:aws:license-manager:eu-east-1:123456789012:license-configuration:lic-edf7f9e241f5e16f29996c842111f448 # optional, arn of the license configuration
  placement:
    hostResourceGroupArn: arn:aws:resource-groups:us-east-1:123456789012:group/my-hrg-name #optional, The ARN of the host resource group in which to launch the instances. If you specify a host resource group ARN, omit the Tenancy parameter or set it to host.
  
```

2. Implement the complete fields from AWS Launch Templates

eg.
```
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  licenseConfiguration: arn:aws:license-manager:eu-east-1:123456789012:license-configuration:lic-edf7f9e241f5e16f29996c842111f448 # optional, arn of the license configuration
  placement:
    affinity: #optional
    availabilityZone: #optional
    groupId: #optional, something to do with placement groups
    groupName: #optional
    hostId: #optional, ID of the dedicated host
    hostResourceGroupArn: arn:aws:resource-groups:us-east-1:123456789012:group/my-hrg-name #optional, The ARN of the host resource group in which to launch the instances. If you specify a host resource group ARN, omit the Tenancy parameter or set it to host.
    paritionNumber: #optional, The number of the partition the instance should launch in. Valid only if the placement group strategy is set to partition.
    spreadDomain: #optional, reserved for future use
    tenancy: dedicated #optional, The tenancy of the instance. An instance with a tenancy of dedicated runs on single-tenant hardware, one of dedicated | default | host

```

3. Implement a simplified API
AWS Launch templates also include extra fields

```
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  dedicatedHostConfig:
    licenseConfiguration: arn:aws:license-manager:eu-east-1:123456789012:license-configuration:lic-edf7f9e241f5e16f29996c842111f448 # optional, arn of the license configuration
    resourceHostGroup: arn:aws:resource-groups:us-east-1:123456789012:group/my-hrg-name #option, arn of the HRG 
```

4. Implement Selectors for all relevant fields

```
apiVersion: karpenter.k8s.aws/v1alpha1
kind: AWSNodeTemplate
metadata:
  name: default
spec:
  licenseSelector:
    name: "myLicense"
  hostResourceGroupSelector:
    name: "myHrg"
```

This would require inexpensive or filterable APIs to query for available license configurations and host resource groups.
Today the recommended apis are:
* `aws license-manager list-license-configurations`
* `aws resource-groups list-groups`
* `aws ec2 describe-placement-groups`


## Recommendations

1. Middle ground solution, stick both to the AWS Launch Template api, but implementing the smallest set of configuration practical, 
limits the amount of checks required to be performed in Karpenter, eg. setting both hostId and HostResourceGroupArn

2. Completely copies the AWS api, would allow the full set of possible configurations supported by AWS. As documented in launch-template-options.md also requires the most work to implement and support.

3. Focuses entirely on the dedicated host feature, ignoring other configuration options. Reduces confusion

## Decision from Working Group meeting

Implement selectors for all relevant fields to improve portability of configuration between clusters / regions / accounts.
