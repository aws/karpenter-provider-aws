## Design Document for Karpenter: Instance Pricing Override via ConfigMap

### Motivation

Karpenter, an open-source project by AWS, is designed to dynamically provision the right compute resources for Kubernetes workloads. Currently, Karpenter selects instance types based on various factors such as performance, availability, and cost. However, users may need to override the default pricing information to reflect custom pricing agreements, on-premises costs, or special discounts. This feature aims to provide flexibility and precision in cost management by allowing users to specify custom pricing per instance type via a ConfigMap.

### Proposed Specification

#### Overview

Introduce a mechanism to override default instance pricing in Karpenter using a Kubernetes ConfigMap. This ConfigMap will allow administrators to specify custom pricing for various instance types, ensuring Karpenter's scheduling and scaling decisions align with the organization's cost structure.

#### ConfigMap Structure

The ConfigMap will contain key-value pairs where the key represents the instance type and the value represents the custom price. The structure of the ConfigMap will be as follows:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: custom-instance-pricing
  namespace: karpenter
data:
  t3.micro:
    price: 0.0092
  m5.large:
    price: 0.096
  c5.xlarge:
    price: 0.17
```

#### Alternative ConfigMap Structure

The ConfigMap will contain nested key-value pairs where the top-level key represents the instance family, cpu generation and the nested keys represent the instance types with their custom prices. The structure of the ConfigMap will be as follows:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: custom-instance-pricing
  namespace: karpenter
data:
  t:
    "3":
      micro:
        price: 0.0092
  m:
    "5":
      large:
        price: 0.096
  c:
    "5":
      xlarge: 
        price: 0.17
```

The benefit of this approach is, that a percentage discount could be attached under the instance family, cpu generation directly making it not necessary to define all instance types.

#### Implementation Details

1. **ConfigMap Creation**: Users will create a ConfigMap named `custom-instance-pricing` in the `karpenter` namespace.
1. **Configuration Loading**: Karpenter will periodically check for updates to the ConfigMap and load the custom pricing data.
1. **Pricing Override Logic**: When making scaling and scheduling decisions, Karpenter will refer to the custom pricing data from the ConfigMap. If an instance type is not specified in the ConfigMap, Karpenter will fall back to the default pricing information.
1. **Validation**: Karpenter will validate the ConfigMap entries to ensure they are in the correct format and contain valid pricing values.
1. **Fallback Mechanism**: In case of errors or if the ConfigMap is missing, Karpenter will log warnings and revert to the default pricing.

### Benefits

1. **Cost Management**: Organizations can tailor the instance pricing to reflect their specific agreements, enabling more accurate cost management.
1. **Flexibility**: Custom pricing allows for better alignment with on-premises costs or special discounts not reflected in the default pricing.
1. **Improved Decision-Making**: More accurate pricing data can lead to better scaling and scheduling decisions, optimizing for cost-efficiency.
2. **Extensability**: Using a custom field "price" in the nested structure allows for additional features later, for example using percentages instead of specific pricing.

### Drawbacks

1. **Complexity**: Introducing custom pricing adds complexity to the configuration and management of Karpenter.
1. **Maintenance Overhead**: Administrators need to ensure the ConfigMap is kept up-to-date with the latest pricing information.
1. **Potential for Misconfiguration**: Errors in the ConfigMap could lead to incorrect pricing data, affecting the cost-efficiency of Karpenter's decisions.

### Risks

1. **Stale Pricing Data**: If the ConfigMap is not updated regularly, Karpenter might make decisions based on outdated pricing, leading to increased costs.
1. **Validation Failures**: Incorrectly formatted ConfigMap entries could cause Karpenter to fall back to default pricing or log errors, impacting its efficiency.
1. **Dependency on ConfigMap Availability**: If the ConfigMap becomes unavailable or corrupted, Karpenter's pricing logic could be compromised, necessitating robust error handling and fallback mechanisms.

### Conclusion

By implementing the ability to override instance pricing via a ConfigMap, Karpenter can provide more flexibility and accuracy in cost management. This feature addresses the need for custom pricing configurations, benefiting organizations with specific pricing agreements or unique cost structures. While it introduces some complexity and potential risks, careful validation and maintenance practices can mitigate these issues, ensuring the feature adds significant value to Karpenter's capabilities.