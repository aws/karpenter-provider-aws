# Background
Karpenter uses the DescribeInstanceTypes when computing memory capacity values but as described [here](https://github.com/aws/karpenter-provider-aws/issues/5676#issuecomment-1958845660) it leaves out the additional overhead that's reserved for the OS and hypervisor. This is currently dealt with via the `vmMemoryOverheadPercent` option which cuts an additional amount of overhead from the value returned by DescribeInstanceTypes. This works for the majority of use-cases and the default value of 7.5% is sufficient for nearly, if not all, instance types but this does create situations of overprovisioning or if a user tunes this value too low, underprovisioning.

For users with a high variance of instance types that want to achieve high utilization ratios or limit cost waste due to overprovisioning, we can improve this calculation logic and a few options are proposed below.

# Solutions
1. Improvements to the DescribeInstanceTypes API - Submit feature requests with the EC2 team and SDK maintainers about providing the overhead values in DescribeInstanceTypes API. If accepted Karpenter could then reliably use these values during scheduling.
2. Use an in-memory cache to store the actual overhead values.
    * Pre-populate values when possible:
      * Known instance types can be handled via similar implementation as [[DRAFT] Add capacity memory overhead generation #4517](https://github.com/aws/karpenter-provider-aws/pull/4517).
      * Unknown instance types such as the case of new ones being introduced can defer to an initial value calculated against vmMemoryOverheadPercent
    * Once a particular instance type has been launched, the cached values are updated with actual.

# Recommendations

Solution 1 would be preferred but may not be feasible in which case Solution 2 can be completely driven by the community
