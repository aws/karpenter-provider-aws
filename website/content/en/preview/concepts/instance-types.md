---
title: "Instance Types"
linkTitle: "Instance Types"
weight: 100

description: >
  Evaluate Instance Type Resources
---
<!-- this document is generated from hack/docs/instancetypes_gen_docs.go -->
AWS instance types offer varying resources and can be selected by labels. The values provided
below are the resources available with some assumptions and after the instance overhead has been subtracted:
- `blockDeviceMappings` are not configured
- `aws-eni-limited-pod-density` is assumed to be `true`
- `amiFamily` is set to the default of `AL2`
## a1 Family
### `a1.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|a|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|a1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|500|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|a1.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1287Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|10|
### `a1.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|a|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|a1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|a1.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|2998Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `a1.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|a|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|a1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|a1.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6561Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `a1.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|a|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|a1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|a1.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14325Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `a1.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|a|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|a1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|a1.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|27322Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `a1.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|a|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|a1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|a1.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|27916Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
## c1 Family
### `c1.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|1740|
 |karpenter.k8s.aws/instance-pods|12|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c1.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|1156Mi|
 |pods|12|
### `c1.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|7168|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c1.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|5912Mi|
 |pods|58|
## c3 Family
### `c3.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|3840|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c3.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3032Mi|
 |pods|29|
### `c3.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|7680|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c3.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6416Mi|
 |pods|58|
### `c3.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|15360|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c3.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|13957Mi|
 |pods|58|
### `c3.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|30720|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c3.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|27077Mi|
 |pods|234|
### `c3.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|61440|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c3.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|57342Mi|
 |pods|234|
## c4 Family
### `c4.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|3840|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c4.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3032Mi|
 |pods|29|
### `c4.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|7680|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c4.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6416Mi|
 |pods|58|
### `c4.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|15360|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c4.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|13957Mi|
 |pods|58|
### `c4.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|30720|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c4.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|25487Mi|
 |pods|234|
### `c4.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|36|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|61440|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c4.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|35840m|
 |ephemeral-storage|17Gi|
 |memory|57343Mi|
 |pods|234|
## c5 Family
### `c5.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|2996Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c5.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6611Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c5.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14478Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c5.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28043Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5.9xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|36|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|73728|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|9xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.9xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|35840m|
 |ephemeral-storage|17Gi|
 |memory|67101Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91271Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5.18xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|72|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|147456|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|18xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.18xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|71750m|
 |ephemeral-storage|17Gi|
 |memory|131929Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c5.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|180269Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c5.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|184387Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c5a Family
### `c5a.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5a.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3156Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c5a.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5a.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6814Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c5a.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5a.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14770Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c5a.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5a.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28747Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5a.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5a.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60570Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5a.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5a.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|92445Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5a.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5a.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|118783Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c5a.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5a.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|182432Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c5ad Family
### `c5ad.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|75|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5ad.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3156Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c5ad.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5ad.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6814Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c5ad.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5ad.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14770Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c5ad.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5ad.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28747Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5ad.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1200|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5ad.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60570Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5ad.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5ad.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|92445Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5ad.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2400|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5ad.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|118773Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c5ad.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5ad.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|182416Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c5d Family
### `c5d.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|50|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|2996Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c5d.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|100|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6611Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c5d.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|200|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14478Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c5d.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|400|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28278Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5d.9xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|36|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|900|
 |karpenter.k8s.aws/instance-memory|73728|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|9xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.9xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|35840m|
 |ephemeral-storage|17Gi|
 |memory|67101Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5d.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91271Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5d.18xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|72|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|147456|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|18xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.18xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|71750m|
 |ephemeral-storage|17Gi|
 |memory|131929Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c5d.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|180269Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c5d.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5d.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|184387Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c5n Family
### `c5n.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|5376|
 |karpenter.k8s.aws/instance-network-bandwidth|3000|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5n.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|4222Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c5n.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|10752|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5n.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|9061Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c5n.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|21504|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5n.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|19380Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c5n.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|43008|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5n.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|38081Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5n.9xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|36|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|9xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5n.9xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|35840m|
 |ephemeral-storage|17Gi|
 |memory|91274Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c5n.18xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|72|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|18xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5n.18xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|71750m|
 |ephemeral-storage|17Gi|
 |memory|180275Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c5n.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|72|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c5n.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|71750m|
 |ephemeral-storage|17Gi|
 |memory|184393Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c6a Family
### `c6a.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3112Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c6a.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6726Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c6a.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14594Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c6a.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28394Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6a.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|59946Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `c6a.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91387Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `c6a.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|117454Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6a.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|180500Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6a.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|243500Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6a.48xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|48xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.48xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|369592Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6a.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6a.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|376259Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c6g Family
### `c6g.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|500|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1321Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `c6g.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3064Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c6g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6694Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c6g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c6g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28444Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60027Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6g.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91660Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|115Gi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6g.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6g.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|119902Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c6gd Family
### `c6gd.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|59|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|500|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1321Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `c6gd.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3064Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c6gd.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6694Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c6gd.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c6gd.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28444Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6gd.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60027Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6gd.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91660Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6gd.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|115Gi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6gd.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|c6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gd.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|119902Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c6gn Family
### `c6gn.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6gn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|1600|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gn.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1321Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `c6gn.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6gn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|3000|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gn.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3064Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c6gn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6gn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|6300|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6694Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c6gn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6gn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c6gn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6gn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28444Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6gn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6gn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60027Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6gn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6gn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91660Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6gn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6gn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6gn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|115Gi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c6i Family
### `c6i.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3135Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c6i.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6772Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c6i.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14684Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c6i.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28572Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6i.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60219Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `c6i.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91916Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `c6i.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|118080Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6i.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|181559Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6i.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|244953Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6i.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6i.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|249047Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c6id Family
### `c6id.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3135Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c6id.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6772Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c6id.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14684Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c6id.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28572Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6id.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60219Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `c6id.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91916Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `c6id.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|118080Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6id.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|5700|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|181559Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6id.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|244953Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6id.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6id.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|249047Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c6in Family
### `c6in.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3135Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c6in.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6772Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c6in.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14684Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c6in.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28572Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c6in.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60219Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `c6in.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91916Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `c6in.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|118080Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6in.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|150000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|181559Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c6in.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|249265Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
### `c6in.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c6in.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|253359Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
## c7g Family
### `c7g.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|520|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1321Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `c7g.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|937|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3064Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c7g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1876|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6694Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c7g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|3750|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c7g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28444Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60027Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7g.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|22500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91660Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|115Gi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `c7g.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7g.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|119898Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c7gd Family
### `c7gd.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|59|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|520|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gd.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1321Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `c7gd.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|937|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gd.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3064Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c7gd.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1876|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gd.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6694Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c7gd.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|3750|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gd.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c7gd.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gd.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28444Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7gd.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gd.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60027Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7gd.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|22500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gd.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91660Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7gd.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gd.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|115Gi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## c7gn Family
### `c7gn.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gn|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gn.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1321Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `c7gn.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gn|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gn.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3064Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `c7gn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gn|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6694Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `c7gn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gn|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `c7gn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gn|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28444Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7gn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gn|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60027Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7gn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gn|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|150000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|91660Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `c7gn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|c|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|c7gn|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|c7gn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|115Gi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## d2 Family
### `d2.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|d2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|31232|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d2.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|29543Mi|
 |pods|58|
### `d2.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|d2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|62464|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d2.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|60213Mi|
 |pods|58|
### `d2.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|d2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|124928|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d2.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|119689Mi|
 |pods|234|
### `d2.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|36|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|d2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d2.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|35840m|
 |ephemeral-storage|17Gi|
 |memory|242668Mi|
 |pods|234|
## d3 Family
### `d3.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|5940|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3000|
 |karpenter.k8s.aws/instance-pods|10|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|31097Mi|
 |pods|10|
 |vpc.amazonaws.com/pod-eni|42|
### `d3.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|11880|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|6000|
 |karpenter.k8s.aws/instance-pods|18|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62835Mi|
 |pods|18|
 |vpc.amazonaws.com/pod-eni|92|
### `d3.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|23760|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|38|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|126369Mi|
 |pods|38|
 |vpc.amazonaws.com/pod-eni|118|
### `d3.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|47520|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|59|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|253544Mi|
 |pods|59|
 |vpc.amazonaws.com/pod-eni|119|
## d3en Family
### `d3en.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|27960|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|6000|
 |karpenter.k8s.aws/instance-pods|10|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3en.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|15184Mi|
 |pods|10|
 |vpc.amazonaws.com/pod-eni|24|
### `d3en.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|55920|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|18|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3en.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|31008Mi|
 |pods|18|
 |vpc.amazonaws.com/pod-eni|58|
### `d3en.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|111840|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|38|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3en.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|62614Mi|
 |pods|38|
 |vpc.amazonaws.com/pod-eni|118|
### `d3en.6xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|24|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|167760|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|40000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|6xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3en.6xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|23870m|
 |ephemeral-storage|17Gi|
 |memory|94272Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|118|
### `d3en.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|223680|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|78|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3en.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|125925Mi|
 |pods|78|
 |vpc.amazonaws.com/pod-eni|118|
### `d3en.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|d|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|d3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|335520|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|89|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|d3en.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|189457Mi|
 |pods|89|
 |vpc.amazonaws.com/pod-eni|119|
## dl1 Family
### `dl1.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|dl|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|dl1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-gpu-count|8|
 |karpenter.k8s.aws/instance-gpu-manufacturer|habana|
 |karpenter.k8s.aws/instance-gpu-memory|32768|
 |karpenter.k8s.aws/instance-gpu-name|gaudi-hl-205|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|4000|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|400000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|dl1.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |habana.ai/gaudi|8|
 |memory|718987Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|62|
## f1 Family
### `f1.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|f|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|f1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|470|
 |karpenter.k8s.aws/instance-memory|124928|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|f1.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|121652Mi|
 |pods|58|
### `f1.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|f|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|f1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|940|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|f1.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|242466Mi|
 |pods|234|
### `f1.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|f|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|f1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|3760|
 |karpenter.k8s.aws/instance-memory|999424|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|394|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|f1.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|978177Mi|
 |pods|394|
## g2 Family
### `g2.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|4096|
 |karpenter.k8s.aws/instance-gpu-name|k520|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|15360|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g2.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|-1761Mi|
 |nvidia.com/gpu|1|
 |pods|58|
### `g2.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-gpu-count|4|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|4096|
 |karpenter.k8s.aws/instance-gpu-name|k520|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|61440|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g2.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|-6001Mi|
 |nvidia.com/gpu|4|
 |pods|234|
## g3 Family
### `g3.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|m60|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|124928|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g3.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|119689Mi|
 |nvidia.com/gpu|1|
 |pods|234|
### `g3.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-gpu-count|2|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|m60|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g3.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|242466Mi|
 |nvidia.com/gpu|2|
 |pods|234|
### `g3.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-gpu-count|4|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|m60|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|499712|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g3.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|482890Mi|
 |nvidia.com/gpu|4|
 |pods|737|
## g3s Family
### `g3s.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g3s|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|m60|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|31232|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g3s.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|29543Mi|
 |nvidia.com/gpu|1|
 |pods|58|
## g4ad Family
### `g4ad.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4ad|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|amd|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|radeon-pro-v520|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2000|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4ad.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |amd.com/gpu|1|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14712Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|12|
### `g4ad.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4ad|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|amd|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|radeon-pro-v520|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|4167|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4ad.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |amd.com/gpu|1|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|29867Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|12|
### `g4ad.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4ad|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|amd|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|radeon-pro-v520|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|8333|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4ad.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |amd.com/gpu|1|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|59946Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|11|
### `g4ad.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4ad|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|2|
 |karpenter.k8s.aws/instance-gpu-manufacturer|amd|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|radeon-pro-v520|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1200|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4ad.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |amd.com/gpu|2|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|120248Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|10|
### `g4ad.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4ad|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|4|
 |karpenter.k8s.aws/instance-gpu-manufacturer|amd|
 |karpenter.k8s.aws/instance-gpu-memory|8192|
 |karpenter.k8s.aws/instance-gpu-name|radeon-pro-v520|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2400|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4ad.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |amd.com/gpu|4|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|239554Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|6|
## g4dn Family
### `g4dn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4dn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|125|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4dn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14975Mi|
 |nvidia.com/gpu|1|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|39|
### `g4dn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4dn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|225|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4dn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30887Mi|
 |nvidia.com/gpu|1|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|39|
### `g4dn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4dn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|225|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4dn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|62713Mi|
 |nvidia.com/gpu|1|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|59|
### `g4dn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4dn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|900|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4dn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|126145Mi|
 |nvidia.com/gpu|1|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|58|
### `g4dn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4dn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|4|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|900|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4dn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|187862Mi|
 |nvidia.com/gpu|4|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `g4dn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4dn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|900|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4dn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|253614Mi|
 |nvidia.com/gpu|1|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|118|
### `g4dn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g4dn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|8|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g4dn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|377769Mi|
 |nvidia.com/gpu|8|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## g5 Family
### `g5.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|24576|
 |karpenter.k8s.aws/instance-gpu-name|a10g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|250|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14771Mi|
 |nvidia.com/gpu|1|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|4|
### `g5.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|24576|
 |karpenter.k8s.aws/instance-gpu-name|a10g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|450|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30684Mi|
 |nvidia.com/gpu|1|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|17|
### `g5.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|24576|
 |karpenter.k8s.aws/instance-gpu-name|a10g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60574Mi|
 |nvidia.com/gpu|1|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|34|
### `g5.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|24576|
 |karpenter.k8s.aws/instance-gpu-name|a10g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|900|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|124325Mi|
 |nvidia.com/gpu|1|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `g5.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|4|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|24576|
 |karpenter.k8s.aws/instance-gpu-name|a10g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|40000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|182444Mi|
 |nvidia.com/gpu|4|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `g5.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|24576|
 |karpenter.k8s.aws/instance-gpu-name|a10g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|251726Mi|
 |nvidia.com/gpu|1|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `g5.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|4|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|24576|
 |karpenter.k8s.aws/instance-gpu-name|a10g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|373698Mi|
 |nvidia.com/gpu|4|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `g5.48xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|g5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|8|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|24576|
 |karpenter.k8s.aws/instance-gpu-name|a10g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|48xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5.48xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|760702Mi|
 |nvidia.com/gpu|8|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|115|
## g5g Family
### `g5g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g5g|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6694Mi|
 |nvidia.com/gpu|1|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `g5g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g5g|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |nvidia.com/gpu|1|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `g5g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g5g|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|28444Mi|
 |nvidia.com/gpu|1|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `g5g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g5g|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|60027Mi|
 |nvidia.com/gpu|1|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `g5g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g5g|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|2|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4g|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|115Gi|
 |nvidia.com/gpu|2|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `g5g.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|g|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|g5g|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|2|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|t4g|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|g5g.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|119902Mi|
 |nvidia.com/gpu|2|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## h1 Family
### `h1.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|h|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|h1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|h1.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|31054Mi|
 |pods|58|
### `h1.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|h|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|h1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|h1.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|61271Mi|
 |pods|234|
### `h1.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|h|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|h1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|h1.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|125732Mi|
 |pods|234|
### `h1.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|h|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|h1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|h1.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|249221Mi|
 |pods|737|
## hpc7g Family
### `hpc7g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|hpc|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|hpc7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|198|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|hpc7g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|118649Mi|
 |pods|198|
### `hpc7g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|hpc|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|hpc7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|198|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|hpc7g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|118649Mi|
 |pods|198|
### `hpc7g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|hpc|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|hpc7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|198|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|hpc7g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|118649Mi|
 |pods|198|
## i2 Family
### `i2.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|31232|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i2.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|29560Mi|
 |pods|58|
### `i2.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|62464|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i2.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|60248Mi|
 |pods|58|
### `i2.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|124928|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i2.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|119761Mi|
 |pods|234|
### `i2.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i2.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|228187Mi|
 |pods|234|
## i3 Family
### `i3.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|475|
 |karpenter.k8s.aws/instance-memory|15616|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14528Mi|
 |pods|29|
### `i3.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|31232|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|29543Mi|
 |pods|58|
### `i3.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|62464|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|60213Mi|
 |pods|58|
### `i3.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|124928|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|119689Mi|
 |pods|234|
### `i3.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|242466Mi|
 |pods|234|
### `i3.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-local-nvme|15200|
 |karpenter.k8s.aws/instance-memory|499712|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|482890Mi|
 |pods|737|
### `i3.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|72|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|i3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|15200|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|71750m|
 |ephemeral-storage|17Gi|
 |memory|506943Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|120|
## i3en Family
### `i3en.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1250|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2100|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3en.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14976Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|5|
### `i3en.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2500|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|4200|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3en.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|12|
### `i3en.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|5000|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|8400|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3en.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62395Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|28|
### `i3en.3xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|12|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7500|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|3xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3en.3xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|11900m|
 |ephemeral-storage|17Gi|
 |memory|94274Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `i3en.6xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|24|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|15000|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|6xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3en.6xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|23870m|
 |ephemeral-storage|17Gi|
 |memory|187868Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `i3en.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|30000|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3en.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|379127Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `i3en.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|60000|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3en.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|756183Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `i3en.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i3en|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|60000|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i3en.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|764534Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## i4g Family
### `i4g.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|468|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4g.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15007Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `i4g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|937|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1875|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30579Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `i4g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1875|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|4687|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62362Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `i4g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3750|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|9375|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124091Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `i4g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7500|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|251320Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `i4g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|15000|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|500447Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## i4i Family
### `i4i.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4i|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|468|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4i.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15005Mi|
 |pods|29|
### `i4i.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4i|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|937|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1875|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4i.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30510Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|6|
### `i4i.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4i|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1875|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|4687|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4i.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62160Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|26|
### `i4i.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4i|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3750|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|9375|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4i.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|123625Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|52|
### `i4i.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4i|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7500|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4i.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|250325Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|112|
### `i4i.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4i|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|15000|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4i.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|498393Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|120|
### `i4i.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4i|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|30000|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4i.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1005782Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|120|
### `i4i.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|i|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|i4i|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|30000|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|i4i.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1022575Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|120|
## im4gn Family
### `im4gn.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|im|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|im4gn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|937|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|im4gn.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7013Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `im4gn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|im|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|im4gn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1875|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|im4gn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `im4gn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|im|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|im4gn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3750|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|im4gn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30381Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `im4gn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|im|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|im4gn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7500|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|im4gn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60029Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `im4gn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|im|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|im4gn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|15000|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|im4gn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123297Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `im4gn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|im|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|im4gn|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|30000|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|im4gn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244199Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## inf1 Family
### `inf1.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|1|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|inf|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|inf1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|38|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|inf1.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|1|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|6804Mi|
 |pods|38|
 |vpc.amazonaws.com/pod-eni|38|
### `inf1.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|1|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|inf|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|inf1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|38|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|inf1.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|1|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|14382Mi|
 |pods|38|
 |vpc.amazonaws.com/pod-eni|38|
### `inf1.6xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|4|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|inf|
 |karpenter.k8s.aws/instance-cpu|24|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|inf1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|49152|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|6xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|inf1.6xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|4|
 |cpu|23870m|
 |ephemeral-storage|17Gi|
 |memory|42536Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `inf1.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|16|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|inf|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|inf1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|321|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|inf1.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|16|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|177976Mi|
 |pods|321|
 |vpc.amazonaws.com/pod-eni|111|
## inf2 Family
### `inf2.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|1|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|inf|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|inf2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2083|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|inf2.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|1|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14162Mi|
 |pods|58|
### `inf2.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|1|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|inf|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|inf2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|16667|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|inf2.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|1|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|118312Mi|
 |pods|234|
### `inf2.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|6|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|inf|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|inf2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|inf2.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|6|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|355262Mi|
 |pods|737|
### `inf2.48xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|12|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|inf|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|inf2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|48xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|inf2.48xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|12|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|718987Mi|
 |pods|737|
## is4gen Family
### `is4gen.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|is|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|is4gen|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|937|
 |karpenter.k8s.aws/instance-memory|6144|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|is4gen.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|5267Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `is4gen.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|is|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|is4gen|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1875|
 |karpenter.k8s.aws/instance-memory|12288|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|is4gen.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|10953Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `is4gen.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|is|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|is4gen|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3750|
 |karpenter.k8s.aws/instance-memory|24576|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|is4gen.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|22473Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `is4gen.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|is|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|is4gen|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7500|
 |karpenter.k8s.aws/instance-memory|49152|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|is4gen.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|46148Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `is4gen.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|is|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|is4gen|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|15000|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|is4gen.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|91664Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `is4gen.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|is|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|is4gen|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|30000|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|is4gen.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|186466Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
## m1 Family
### `m1.small`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|1740|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|small|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m1.small|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1200Mi|
 |pods|8|
### `m1.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|3788|
 |karpenter.k8s.aws/instance-pods|12|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m1.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|3222Mi|
 |pods|12|
### `m1.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|7680|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m1.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6735Mi|
 |pods|29|
### `m1.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|15360|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m1.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|13957Mi|
 |pods|58|
## m2 Family
### `m2.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|17510|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m2.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|16036Mi|
 |pods|58|
### `m2.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|35020|
 |karpenter.k8s.aws/instance-pods|118|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m2.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|32537Mi|
 |pods|118|
### `m2.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|70041|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m2.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|65687Mi|
 |pods|234|
## m3 Family
### `m3.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|3840|
 |karpenter.k8s.aws/instance-pods|12|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m3.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|3219Mi|
 |pods|12|
### `m3.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|7680|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m3.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6735Mi|
 |pods|29|
### `m3.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|15360|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m3.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|13957Mi|
 |pods|58|
### `m3.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|30720|
 |karpenter.k8s.aws/instance-pods|118|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m3.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|28380Mi|
 |pods|118|
## m4 Family
### `m4.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-pods|20|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m4.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7337Mi|
 |pods|20|
### `m4.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m4.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14964Mi|
 |pods|58|
### `m4.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m4.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|31054Mi|
 |pods|58|
### `m4.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m4.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|61271Mi|
 |pods|234|
### `m4.10xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|40|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|163840|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|10xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m4.10xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|39830m|
 |ephemeral-storage|17Gi|
 |memory|158063Mi|
 |pods|234|
### `m4.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m4.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|239554Mi|
 |pods|234|
## m5 Family
### `m5.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6931Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m5.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14479Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m5.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30568Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m5.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|59753Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|124209Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|185745Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|246145Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|373652Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|377769Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|120|
## m5a Family
### `m5a.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5a.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6977Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m5a.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5a.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m5a.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5a.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30394Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m5a.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5a.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60108Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5a.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5a.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123474Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5a.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5a.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186740Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5a.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5a.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244473Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5a.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5a.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|371005Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m5ad Family
### `m5ad.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|75|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5ad.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6977Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m5ad.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5ad.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m5ad.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5ad.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30394Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m5ad.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5ad.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60108Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5ad.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1200|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5ad.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123474Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5ad.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5ad.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186740Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5ad.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2400|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5ad.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244473Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5ad.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5ad.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|371005Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m5d Family
### `m5d.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|75|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6931Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m5d.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14479Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m5d.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30568Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m5d.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60458Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5d.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1200|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|124209Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5d.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|185745Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5d.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2400|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|246145Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5d.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|369418Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5d.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5d.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|377769Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m5dn Family
### `m5dn.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|75|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|2100|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7019Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m5dn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|4100|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14656Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m5dn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|8125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30568Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m5dn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|16250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60458Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5dn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1200|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|124209Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5dn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|187862Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5dn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2400|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|246145Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5dn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|355262Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5dn.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5dn.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|373652Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m5n Family
### `m5n.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|2100|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7019Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m5n.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|4100|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14656Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m5n.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|8125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30568Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m5n.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|16250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60458Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5n.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|124209Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5n.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|187862Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m5n.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|246145Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5n.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|373652Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5n.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5n.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|377769Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m5zn Family
### `m5zn.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5zn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|3000|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5zn.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6931Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|13|
### `m5zn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5zn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5zn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14479Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|29|
### `m5zn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5zn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5zn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30215Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|62|
### `m5zn.3xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|12|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5zn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|49152|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|3xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5zn.3xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|11900m|
 |ephemeral-storage|17Gi|
 |memory|43993Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|92|
### `m5zn.6xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|24|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5zn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|6xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5zn.6xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|23870m|
 |ephemeral-storage|17Gi|
 |memory|91277Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `m5zn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5zn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5zn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|180281Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m5zn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m5zn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m5zn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|184399Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m6a Family
### `m6a.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7046Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m6a.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14595Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m6a.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30331Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m6a.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|59868Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6a.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|122995Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `m6a.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|185860Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `m6a.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|243553Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6a.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|369649Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6a.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|495698Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6a.48xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|48xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.48xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|747889Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6a.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6a.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|761436Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m6g Family
### `m6g.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|500|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|3278Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `m6g.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6979Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m6g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14522Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m6g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30249Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m6g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|59764Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|122769Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6g.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|185671Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|243143Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6g.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6g.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|248453Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m6gd Family
### `m6gd.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|59|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|500|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|3278Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `m6gd.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6979Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m6gd.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14522Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m6gd.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30249Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m6gd.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|59764Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6gd.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|122769Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6gd.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|185671Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6gd.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|243143Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6gd.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|m6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6gd.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|248453Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m6i Family
### `m6i.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7092Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m6i.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14685Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m6i.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30509Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m6i.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60223Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6i.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123621Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `m6i.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186919Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `m6i.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244784Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6i.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|371766Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6i.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|476504Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6i.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6i.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|498562Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m6id Family
### `m6id.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7092Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m6id.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14685Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m6id.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30509Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m6id.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60223Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6id.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123621Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `m6id.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186919Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `m6id.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244784Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6id.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|5700|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|371766Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6id.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|498562Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6id.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6id.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|506890Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m6idn Family
### `m6idn.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7092Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m6idn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14685Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m6idn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30509Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m6idn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60223Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6idn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123621Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `m6idn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186919Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `m6idn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244784Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6idn.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|5700|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|150000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|371766Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6idn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|480816Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
### `m6idn.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6idn.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|502874Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
## m6in Family
### `m6in.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7092Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m6in.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14685Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m6in.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30509Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m6in.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60223Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m6in.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123621Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `m6in.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186919Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `m6in.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244784Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6in.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|150000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|371766Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m6in.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|502874Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
### `m6in.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m6in.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|511202Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
## m7a Family
### `m7a.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|390|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|3326Mi|
 |pods|8|
### `m7a.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7029Mi|
 |pods|29|
### `m7a.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14578Mi|
 |pods|58|
### `m7a.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30314Mi|
 |pods|58|
### `m7a.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|59851Mi|
 |pods|234|
### `m7a.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|122897Mi|
 |pods|234|
### `m7a.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|185843Mi|
 |pods|234|
### `m7a.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|243354Mi|
 |pods|737|
### `m7a.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|369448Mi|
 |pods|737|
### `m7a.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|495702Mi|
 |pods|737|
### `m7a.48xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|48xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.48xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|747889Mi|
 |pods|737|
### `m7a.metal-48xl`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7a|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal-48xl|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7a.metal-48xl|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|761381Mi|
 |pods|737|
## m7g Family
### `m7g.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|520|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|3295Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `m7g.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|937|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7013Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m7g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1876|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m7g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3750|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30381Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m7g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60029Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m7g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123297Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m7g.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|22500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186464Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m7g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244199Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `m7g.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7g.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|248449Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m7gd Family
### `m7gd.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|59|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|520|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7gd.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|3295Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `m7gd.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|937|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7gd.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7013Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `m7gd.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1876|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7gd.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14589Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `m7gd.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3750|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7gd.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30381Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `m7gd.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7gd.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60029Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m7gd.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7gd.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123297Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m7gd.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|22500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7gd.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186464Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `m7gd.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7gd.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244199Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## m7i Family
### `m7i.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7075Mi|
 |pods|29|
### `m7i.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14668Mi|
 |pods|58|
### `m7i.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30492Mi|
 |pods|58|
### `m7i.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60206Mi|
 |pods|234|
### `m7i.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123604Mi|
 |pods|234|
### `m7i.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|186902Mi|
 |pods|234|
### `m7i.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|244767Mi|
 |pods|737|
### `m7i.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|371564Mi|
 |pods|737|
### `m7i.48xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|48xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i.48xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|752123Mi|
 |pods|737|
## m7i-flex Family
### `m7i-flex.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i-flex|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|390|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i-flex.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7075Mi|
 |pods|29|
### `m7i-flex.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i-flex|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i-flex.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14668Mi|
 |pods|58|
### `m7i-flex.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i-flex|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i-flex.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30492Mi|
 |pods|58|
### `m7i-flex.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i-flex|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i-flex.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|60206Mi|
 |pods|234|
### `m7i-flex.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|m|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|m7i-flex|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|m7i-flex.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|123604Mi|
 |pods|234|
## p2 Family
### `p2.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|p2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|12288|
 |karpenter.k8s.aws/instance-gpu-name|k80|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|62464|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p2.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|56786Mi|
 |nvidia.com/gpu|1|
 |pods|58|
### `p2.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|p2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-gpu-count|8|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|12288|
 |karpenter.k8s.aws/instance-gpu-name|k80|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|499712|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p2.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|459304Mi|
 |nvidia.com/gpu|8|
 |pods|234|
### `p2.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|p2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-gpu-count|16|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|12288|
 |karpenter.k8s.aws/instance-gpu-name|k80|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|749568|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p2.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|690421Mi|
 |nvidia.com/gpu|16|
 |pods|234|
## p3 Family
### `p3.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|p3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-gpu-count|1|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|v100|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|62464|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p3.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|60213Mi|
 |nvidia.com/gpu|1|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `p3.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|p3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-gpu-count|4|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|v100|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p3.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|242466Mi|
 |nvidia.com/gpu|4|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `p3.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|p3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-gpu-count|8|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|16384|
 |karpenter.k8s.aws/instance-gpu-name|v100|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|499712|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p3.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|459304Mi|
 |nvidia.com/gpu|8|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
## p3dn Family
### `p3dn.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|p3dn|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-gpu-count|8|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|32768|
 |karpenter.k8s.aws/instance-gpu-name|v100|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p3dn.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|718987Mi|
 |nvidia.com/gpu|8|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## p4d Family
### `p4d.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|p4d|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-gpu-count|8|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|40960|
 |karpenter.k8s.aws/instance-gpu-name|a100|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|8000|
 |karpenter.k8s.aws/instance-memory|1179648|
 |karpenter.k8s.aws/instance-network-bandwidth|400000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p4d.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|1082712Mi|
 |nvidia.com/gpu|8|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|62|
## p5 Family
### `p5.48xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|p|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|p5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-gpu-count|8|
 |karpenter.k8s.aws/instance-gpu-manufacturer|nvidia|
 |karpenter.k8s.aws/instance-gpu-memory|81920|
 |karpenter.k8s.aws/instance-gpu-name|h100|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|30400|
 |karpenter.k8s.aws/instance-memory|2097152|
 |karpenter.k8s.aws/instance-network-bandwidth|3200000|
 |karpenter.k8s.aws/instance-pods|100|
 |karpenter.k8s.aws/instance-size|48xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|p5.48xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|2044045Mi|
 |nvidia.com/gpu|8|
 |pods|100|
 |vpc.amazonaws.com/pod-eni|120|
## r3 Family
### `r3.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|15360|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r3.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14548Mi|
 |pods|29|
### `r3.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|31232|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r3.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|29560Mi|
 |pods|58|
### `r3.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|62464|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r3.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|60248Mi|
 |pods|58|
### `r3.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|124928|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r3.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|119761Mi|
 |pods|234|
### `r3.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r3.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|242814Mi|
 |pods|234|
## r4 Family
### `r4.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|15616|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r4.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14528Mi|
 |pods|29|
### `r4.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|31232|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r4.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|29543Mi|
 |pods|58|
### `r4.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|62464|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r4.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|60213Mi|
 |pods|58|
### `r4.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|124928|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r4.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|119689Mi|
 |pods|234|
### `r4.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r4.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|242466Mi|
 |pods|234|
### `r4.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r4|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|499712|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r4.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|482890Mi|
 |pods|737|
## r5 Family
### `r5.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14976Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r5.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r5.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62395Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r5.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124213Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|251619Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|379127Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|501165Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|756183Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|764534Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|120|
## r5a Family
### `r5a.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5a.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15022Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r5a.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5a.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30659Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r5a.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5a.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62574Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r5a.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5a.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124568Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5a.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5a.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|252396Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5a.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5a.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|380123Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5a.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5a.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|502316Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5a.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5a|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5a.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|757770Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r5ad Family
### `r5ad.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|75|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5ad.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15022Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r5ad.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5ad.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30659Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r5ad.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5ad.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62574Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r5ad.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5ad.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124568Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5ad.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1200|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5ad.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|252396Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5ad.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5ad.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|380123Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5ad.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2400|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5ad.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|502316Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5ad.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5ad|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5ad.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|757770Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r5b Family
### `r5b.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14976Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r5b.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r5b.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62395Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r5b.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124213Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5b.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|251619Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5b.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|379127Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5b.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|501165Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5b.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|756183Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5b.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5b|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5b.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|764534Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r5d Family
### `r5d.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|75|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14976Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r5d.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r5d.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62395Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r5d.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124213Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5d.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1200|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|251619Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5d.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|379127Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5d.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2400|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|501165Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5d.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|756183Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5d.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r5d|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5d.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|764534Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r5dn Family
### `r5dn.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|75|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2100|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14976Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r5dn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|4100|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r5dn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|8125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62395Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r5dn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|600|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|16250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124213Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5dn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1200|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|251619Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5dn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|379127Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5dn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2400|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|501165Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5dn.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|756183Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5dn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5dn|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3600|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5dn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|764534Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r5n Family
### `r5n.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|2100|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14976Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r5n.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|4100|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r5n.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|8125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62395Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r5n.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|16250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124213Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5n.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|251619Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5n.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|379127Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r5n.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|501165Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5n.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|756183Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r5n.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r5n|
 |karpenter.k8s.aws/instance-generation|5|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r5n.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|764534Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r6a Family
### `r6a.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14915Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r6a.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30332Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r6a.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|61805Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r6a.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|122918Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6a.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|249095Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `r6a.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|375007Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `r6a.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|495752Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6a.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|747943Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6a.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1000095Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6a.48xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1572864|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|48xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.48xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|1504476Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6a.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|192|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6a|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|1572864|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6a.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|191450m|
 |ephemeral-storage|17Gi|
 |memory|1527556Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r6g Family
### `r6g.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|500|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|7292Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `r6g.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15007Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r6g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30579Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r6g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62362Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r6g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124091Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|251320Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6g.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|378649Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|500447Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6g.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6g|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6g.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|505757Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r6gd Family
### `r6gd.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|59|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|500|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|7292Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `r6gd.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15007Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r6gd.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30579Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r6gd.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62362Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r6gd.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|124091Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6gd.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|251320Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6gd.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|378649Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6gd.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|500447Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6gd.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|r6gd|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6gd.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|505757Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r6i Family
### `r6i.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15005Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r6i.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30510Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r6i.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62160Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r6i.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|123625Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6i.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|250325Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `r6i.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|377126Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `r6i.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|498393Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6i.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|752180Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6i.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1005782Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6i.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6i|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6i.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1022575Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r6id Family
### `r6id.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|781|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15005Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r6id.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1562|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30510Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r6id.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62160Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r6id.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|123625Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6id.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|250325Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `r6id.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|18750|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|377126Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `r6id.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|498393Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6id.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|5700|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|37500|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|752180Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6id.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1005782Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6id.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6id|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6id.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1022575Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r6idn Family
### `r6idn.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15005Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r6idn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30510Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r6idn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62160Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r6idn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|123625Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6idn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|250325Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `r6idn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|377126Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `r6idn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|498393Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6idn.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|5700|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|150000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|752180Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6idn.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1010094Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
### `r6idn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6idn|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6idn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1026887Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
## r6in Family
### `r6in.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|15005Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r6in.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30510Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r6in.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62160Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r6in.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|123625Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r6in.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|250325Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|84|
### `r6in.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|377126Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `r6in.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|498393Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6in.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|150000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|752180Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r6in.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1010094Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
### `r6in.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r6in|
 |karpenter.k8s.aws/instance-generation|6|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|200000|
 |karpenter.k8s.aws/instance-pods|345|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r6in.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1026887Mi|
 |pods|345|
 |vpc.amazonaws.com/pod-eni|108|
## r7g Family
### `r7g.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|520|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|7244Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `r7g.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|937|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14908Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r7g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1876|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30381Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r7g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|3750|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|61966Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r7g.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|123299Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r7g.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|249735Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r7g.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|22500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|376273Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r7g.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|497278Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `r7g.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7g|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7g.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|505754Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## r7gd Family
### `r7gd.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|59|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|520|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7gd.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|7244Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|4|
### `r7gd.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|937|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7gd.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14908Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `r7gd.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1876|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7gd.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30381Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `r7gd.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|3750|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7gd.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|61966Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `r7gd.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|7500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7gd.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|123299Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r7gd.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7gd.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|178874Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r7gd.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|22500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7gd.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|239495Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `r7gd.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|r|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|r7gd|
 |karpenter.k8s.aws/instance-generation|7|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|30000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|r7gd.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|497278Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## t1 Family
### `t1.micro`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|627|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|micro|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t1.micro|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|138Mi|
 |pods|4|
## t2 Family
### `t2.nano`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|512|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|nano|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t2.nano|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|39Mi|
 |pods|4|
### `t2.micro`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|1024|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|micro|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t2.micro|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|542Mi|
 |pods|4|
### `t2.small`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-pods|11|
 |karpenter.k8s.aws/instance-size|small|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t2.small|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|1471Mi|
 |pods|11|
### `t2.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-pods|17|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t2.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3348Mi|
 |pods|17|
### `t2.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-pods|35|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t2.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7172Mi|
 |pods|35|
### `t2.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-pods|44|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t2.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|15118Mi|
 |pods|44|
### `t2.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t2|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-pods|44|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t2.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|31208Mi|
 |pods|44|
## t3 Family
### `t3.nano`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|512|
 |karpenter.k8s.aws/instance-network-bandwidth|32|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|nano|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3.nano|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|19Mi|
 |pods|4|
### `t3.micro`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1024|
 |karpenter.k8s.aws/instance-network-bandwidth|64|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|micro|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3.micro|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|515Mi|
 |pods|4|
### `t3.small`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|128|
 |karpenter.k8s.aws/instance-pods|11|
 |karpenter.k8s.aws/instance-size|small|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3.small|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|1434Mi|
 |pods|11|
### `t3.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|256|
 |karpenter.k8s.aws/instance-pods|17|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3288Mi|
 |pods|17|
### `t3.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|512|
 |karpenter.k8s.aws/instance-pods|35|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7068Mi|
 |pods|35|
### `t3.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1024|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14771Mi|
 |pods|58|
### `t3.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2048|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30684Mi|
 |pods|58|
## t3a Family
### `t3a.nano`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3a|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|512|
 |karpenter.k8s.aws/instance-network-bandwidth|32|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|nano|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3a.nano|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|21Mi|
 |pods|4|
### `t3a.micro`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3a|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1024|
 |karpenter.k8s.aws/instance-network-bandwidth|64|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|micro|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3a.micro|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|520Mi|
 |pods|4|
### `t3a.small`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3a|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|128|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|small|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3a.small|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|1473Mi|
 |pods|8|
### `t3a.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3a|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|256|
 |karpenter.k8s.aws/instance-pods|17|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3a.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3300Mi|
 |pods|17|
### `t3a.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3a|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|512|
 |karpenter.k8s.aws/instance-pods|35|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3a.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|7091Mi|
 |pods|35|
### `t3a.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3a|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1024|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3a.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14817Mi|
 |pods|58|
### `t3a.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t3a|
 |karpenter.k8s.aws/instance-generation|3|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2048|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t3a.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30774Mi|
 |pods|58|
## t4g Family
### `t4g.nano`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|512|
 |karpenter.k8s.aws/instance-network-bandwidth|32|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|nano|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t4g.nano|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|-45Mi|
 |pods|4|
### `t4g.micro`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1024|
 |karpenter.k8s.aws/instance-network-bandwidth|64|
 |karpenter.k8s.aws/instance-pods|4|
 |karpenter.k8s.aws/instance-size|micro|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t4g.micro|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|452Mi|
 |pods|4|
### `t4g.small`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|2048|
 |karpenter.k8s.aws/instance-network-bandwidth|128|
 |karpenter.k8s.aws/instance-pods|11|
 |karpenter.k8s.aws/instance-size|small|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t4g.small|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|1301Mi|
 |pods|11|
### `t4g.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|4096|
 |karpenter.k8s.aws/instance-network-bandwidth|256|
 |karpenter.k8s.aws/instance-pods|17|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t4g.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|3221Mi|
 |pods|17|
### `t4g.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|8192|
 |karpenter.k8s.aws/instance-network-bandwidth|512|
 |karpenter.k8s.aws/instance-pods|35|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t4g.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|6995Mi|
 |pods|35|
### `t4g.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|1024|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t4g.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|14688Mi|
 |pods|58|
### `t4g.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|t|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|t4g|
 |karpenter.k8s.aws/instance-generation|4|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|2048|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|t4g.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|30579Mi|
 |pods|58|
## trn1 Family
### `trn1.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|1|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|trn|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|trn1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|474|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|3125|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|trn1.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|1|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|29317Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|17|
### `trn1.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|16|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|trn|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|trn1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|800000|
 |karpenter.k8s.aws/instance-pods|247|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|trn1.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|16|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|481894Mi|
 |pods|247|
 |vpc.amazonaws.com/pod-eni|82|
## trn1n Family
### `trn1n.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-accelerator-count|16|
 |karpenter.k8s.aws/instance-accelerator-manufacturer|aws|
 |karpenter.k8s.aws/instance-accelerator-name|inferentia|
 |karpenter.k8s.aws/instance-category|trn|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|trn1n|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|7600|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|1600000|
 |karpenter.k8s.aws/instance-pods|247|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|trn1n.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |aws.amazon.com/neuron|16|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|481894Mi|
 |pods|247|
 |vpc.amazonaws.com/pod-eni|120|
## u-12tb1 Family
### `u-12tb1.112xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|u|
 |karpenter.k8s.aws/instance-cpu|448|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|u-12tb1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|12582912|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|112xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|u-12tb1.112xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|446810m|
 |ephemeral-storage|17Gi|
 |memory|11630731Mi|
 |pods|737|
## u-18tb1 Family
### `u-18tb1.112xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|u|
 |karpenter.k8s.aws/instance-cpu|448|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|u-18tb1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|18874368|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|112xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|u-18tb1.112xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|446810m|
 |ephemeral-storage|17Gi|
 |memory|17450328Mi|
 |pods|737|
## u-24tb1 Family
### `u-24tb1.112xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|u|
 |karpenter.k8s.aws/instance-cpu|448|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|u-24tb1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|25165824|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|112xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|u-24tb1.112xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|446810m|
 |ephemeral-storage|17Gi|
 |memory|23269925Mi|
 |pods|737|
## u-3tb1 Family
### `u-3tb1.56xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|u|
 |karpenter.k8s.aws/instance-cpu|224|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|u-3tb1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|3145728|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|56xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|u-3tb1.56xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|223370m|
 |ephemeral-storage|17Gi|
 |memory|2906869Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|6|
## u-6tb1 Family
### `u-6tb1.56xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|u|
 |karpenter.k8s.aws/instance-cpu|224|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|u-6tb1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|6291456|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|56xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|u-6tb1.56xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|223370m|
 |ephemeral-storage|17Gi|
 |memory|5811134Mi|
 |pods|737|
### `u-6tb1.112xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|u|
 |karpenter.k8s.aws/instance-cpu|448|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|u-6tb1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|6291456|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|112xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|u-6tb1.112xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|446810m|
 |ephemeral-storage|17Gi|
 |memory|5811134Mi|
 |pods|737|
## u-9tb1 Family
### `u-9tb1.112xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|u|
 |karpenter.k8s.aws/instance-cpu|448|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|u-9tb1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|9437184|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|112xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|u-9tb1.112xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|446810m|
 |ephemeral-storage|17Gi|
 |memory|8720933Mi|
 |pods|737|
## vt1 Family
### `vt1.3xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|vt|
 |karpenter.k8s.aws/instance-cpu|12|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|vt1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|24576|
 |karpenter.k8s.aws/instance-network-bandwidth|3120|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|3xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|vt1.3xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|11900m|
 |ephemeral-storage|17Gi|
 |memory|21410Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `vt1.6xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|vt|
 |karpenter.k8s.aws/instance-cpu|24|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|vt1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|49152|
 |karpenter.k8s.aws/instance-network-bandwidth|6250|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|6xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|vt1.6xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|23870m|
 |ephemeral-storage|17Gi|
 |memory|43064Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `vt1.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|vt|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|vt1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|vt1.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|178417Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## x1 Family
### `x1.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|999424|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x1.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|979937Mi|
 |pods|234|
### `x1.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x1|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|1998848|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x1.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|1962963Mi|
 |pods|234|
## x1e Family
### `x1e.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x1e|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|124928|
 |karpenter.k8s.aws/instance-network-bandwidth|625|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x1e.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|121971Mi|
 |pods|29|
### `x1e.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x1e|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|249856|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x1e.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|244429Mi|
 |pods|58|
### `x1e.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x1e|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|499712|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x1e.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|490159Mi|
 |pods|58|
### `x1e.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x1e|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|999424|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x1e.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|981470Mi|
 |pods|58|
### `x1e.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x1e|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|1998848|
 |karpenter.k8s.aws/instance-network-bandwidth|10000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x1e.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|1962964Mi|
 |pods|234|
### `x1e.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x1e|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|xen|
 |karpenter.k8s.aws/instance-memory|3997696|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x1e.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|3929018Mi|
 |pods|234|
## x2gd Family
### `x2gd.medium`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|1|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|59|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|500|
 |karpenter.k8s.aws/instance-pods|8|
 |karpenter.k8s.aws/instance-size|medium|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.medium|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|940m|
 |ephemeral-storage|17Gi|
 |memory|15238Mi|
 |pods|8|
 |vpc.amazonaws.com/pod-eni|10|
### `x2gd.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|30898Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|9|
### `x2gd.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|62363Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|18|
### `x2gd.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|475|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|126028Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|38|
### `x2gd.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|251322Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `x2gd.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|505983Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `x2gd.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|20000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|760444Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `x2gd.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|1009539Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `x2gd.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|x2gd|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|arm64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2gd.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|1020165Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## x2idn Family
### `x2idn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2idn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2idn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|1013878Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `x2idn.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2idn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|1572864|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2idn.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|1525709Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `x2idn.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2idn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|2097152|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2idn.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|2037154Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `x2idn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2idn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|2097152|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2idn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|2053948Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## x2iedn Family
### `x2iedn.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iedn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|118|
 |karpenter.k8s.aws/instance-memory|131072|
 |karpenter.k8s.aws/instance-network-bandwidth|1875|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iedn.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|126622Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|11|
### `x2iedn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iedn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|237|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iedn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|255442Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|27|
### `x2iedn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iedn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|475|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iedn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|510288Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `x2iedn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iedn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|950|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iedn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|1023652Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `x2iedn.16xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|64|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iedn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1900|
 |karpenter.k8s.aws/instance-memory|2097152|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|16xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iedn.16xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|63770m|
 |ephemeral-storage|17Gi|
 |memory|2045251Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `x2iedn.24xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|96|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iedn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|2850|
 |karpenter.k8s.aws/instance-memory|3145728|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|24xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iedn.24xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|95690m|
 |ephemeral-storage|17Gi|
 |memory|3072767Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `x2iedn.32xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iedn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|4194304|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|32xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iedn.32xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|4099899Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `x2iedn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|128|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iedn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|3800|
 |karpenter.k8s.aws/instance-memory|4194304|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iedn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|127610m|
 |ephemeral-storage|17Gi|
 |memory|4116693Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## x2iezn Family
### `x2iezn.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iezn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|262144|
 |karpenter.k8s.aws/instance-network-bandwidth|12500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iezn.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|253676Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|62|
### `x2iezn.4xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|16|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iezn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|524288|
 |karpenter.k8s.aws/instance-network-bandwidth|15000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|4xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iezn.4xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|15890m|
 |ephemeral-storage|17Gi|
 |memory|506758Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `x2iezn.6xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|24|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iezn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|786432|
 |karpenter.k8s.aws/instance-network-bandwidth|50000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|6xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iezn.6xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|23870m|
 |ephemeral-storage|17Gi|
 |memory|761577Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `x2iezn.8xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|32|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iezn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1048576|
 |karpenter.k8s.aws/instance-network-bandwidth|75000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|8xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iezn.8xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|31850m|
 |ephemeral-storage|17Gi|
 |memory|1016978Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|114|
### `x2iezn.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iezn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-memory|1572864|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iezn.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|1521487Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `x2iezn.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|x|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|true|
 |karpenter.k8s.aws/instance-family|x2iezn|
 |karpenter.k8s.aws/instance-generation|2|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-memory|1572864|
 |karpenter.k8s.aws/instance-network-bandwidth|100000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|x2iezn.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|1538075Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
## z1d Family
### `z1d.large`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|z|
 |karpenter.k8s.aws/instance-cpu|2|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|z1d|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|75|
 |karpenter.k8s.aws/instance-memory|16384|
 |karpenter.k8s.aws/instance-network-bandwidth|750|
 |karpenter.k8s.aws/instance-pods|29|
 |karpenter.k8s.aws/instance-size|large|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|z1d.large|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|1930m|
 |ephemeral-storage|17Gi|
 |memory|14976Mi|
 |pods|29|
 |vpc.amazonaws.com/pod-eni|13|
### `z1d.xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|z|
 |karpenter.k8s.aws/instance-cpu|4|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|z1d|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|150|
 |karpenter.k8s.aws/instance-memory|32768|
 |karpenter.k8s.aws/instance-network-bandwidth|1250|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|z1d.xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|3920m|
 |ephemeral-storage|17Gi|
 |memory|30569Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|28|
### `z1d.2xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|z|
 |karpenter.k8s.aws/instance-cpu|8|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|z1d|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|300|
 |karpenter.k8s.aws/instance-memory|65536|
 |karpenter.k8s.aws/instance-network-bandwidth|2500|
 |karpenter.k8s.aws/instance-pods|58|
 |karpenter.k8s.aws/instance-size|2xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|z1d.2xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|7910m|
 |ephemeral-storage|17Gi|
 |memory|62395Mi|
 |pods|58|
 |vpc.amazonaws.com/pod-eni|58|
### `z1d.3xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|z|
 |karpenter.k8s.aws/instance-cpu|12|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|z1d|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|450|
 |karpenter.k8s.aws/instance-memory|98304|
 |karpenter.k8s.aws/instance-network-bandwidth|5000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|3xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|z1d.3xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|11900m|
 |ephemeral-storage|17Gi|
 |memory|92338Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `z1d.6xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|z|
 |karpenter.k8s.aws/instance-cpu|24|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|z1d|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|900|
 |karpenter.k8s.aws/instance-memory|196608|
 |karpenter.k8s.aws/instance-network-bandwidth|12000|
 |karpenter.k8s.aws/instance-pods|234|
 |karpenter.k8s.aws/instance-size|6xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|z1d.6xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|23870m|
 |ephemeral-storage|17Gi|
 |memory|187868Mi|
 |pods|234|
 |vpc.amazonaws.com/pod-eni|54|
### `z1d.12xlarge`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|z|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|z1d|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor|nitro|
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|12xlarge|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|z1d.12xlarge|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|373663Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
### `z1d.metal`
#### Labels
 | Label | Value |
 |--|--|
 |karpenter.k8s.aws/instance-category|z|
 |karpenter.k8s.aws/instance-cpu|48|
 |karpenter.k8s.aws/instance-encryption-in-transit-supported|false|
 |karpenter.k8s.aws/instance-family|z1d|
 |karpenter.k8s.aws/instance-generation|1|
 |karpenter.k8s.aws/instance-hypervisor||
 |karpenter.k8s.aws/instance-local-nvme|1800|
 |karpenter.k8s.aws/instance-memory|393216|
 |karpenter.k8s.aws/instance-network-bandwidth|25000|
 |karpenter.k8s.aws/instance-pods|737|
 |karpenter.k8s.aws/instance-size|metal|
 |kubernetes.io/arch|amd64|
 |kubernetes.io/os|linux|
 |node.kubernetes.io/instance-type|z1d.metal|
#### Resources
 | Resource | Quantity |
 |--|--|
 |cpu|47810m|
 |ephemeral-storage|17Gi|
 |memory|377782Mi|
 |pods|737|
 |vpc.amazonaws.com/pod-eni|107|
