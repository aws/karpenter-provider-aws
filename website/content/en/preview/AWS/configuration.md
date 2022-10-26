---
title: "Configuration"
linkTitle: "Configuration"
weight: 5
description: >
  Configure Karpenter on AWS
---

There are two main configuration mechanisms that can be used to configure Karpenter: Environment Variables / CLI parameters to the controller and webhook binaries and the `karpenter-global-settings` config-map.

## ConfigMap

Karpenter installs a default configuration via its Helm chart that should work for most.  Additional configuration can be performed by editing the `karpenter-global-settings` configmap within the namespace that Karpenter was installed in.

For more details on common Karpenter settings, see [Karpenter Global Configuration](../../tasks/configuration/#configmap)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: karpenter-global-settings
  namespace: karpenter
data:
  ...
  # Any global tag value can be specified by including the "aws.tags.<tag-key>" prefix
  # associated with the value in the key-value tag pair
  aws.tags.custom-tag: custom-tag-value 
  aws.tags.custom-tag2: custom-tag-value
```

#### `aws.tags.<tag-key>`

Global tags are applied to __all__ AWS infrastructure resources deployed by Karpenter. These resources include:

- Launch Templates
- Volumes
- Instances

{{% alert title="Note" color="primary" %}}
Since you can specify tags at the global level and in the `AWSNodeTemplate` resource, if a key is specified in both locations, the `AWSNodeTemplate` tag value will override the global tag.
{{% /alert %}}