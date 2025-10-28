# Variable Templating for `userData` in EC2NodeClass

## Overview

This proposal addresses the need for users, particularly those using "Custom" AMIFamily, to include both Karpenter-managed values and custom data in their `EC2NodeClass`'s `userData` through templating variables.

Here we address two main problems:

1. Users must hardcode cluster-specific information (cluster endpoint, node labels, kubelet configuration) in their `userData`, or allow the AMI itself to obtain this information from outside the cluster, whereas non-Custom AMI families are able to access this information automatically.
2. Any change to the `userData` field triggers drift and node replacement, which is undesired for the case where the user is passing rotating data to the `userData` field.

This proposal introduces a variable templating system that allows Custom AMI users to access Karpenter-managed values automatically while also being able to provide user-defined and potentially non-drifting `userData` templating variables to be referenced in the `userData` field.

## User Stories

- As a Custom AMI user, I want Karpenter to be able to natively provide node-specific values (taints, labels, etc.) to my userData configuration automatically, without extra handling on the AMI side to pull this information.
- As a cluster manager, I want to define reusable template variables to avoid hardcoding values directly in the `userData` field.
- As a cluster manager, I want granular control over which `userData` changes cause drift versus which do not.

## Background

### Standard AMI Families vs. Custom

For standard AMI families like AL2 and Bottlerocket, Karpenter automatically generates `userData` that includes cluster-specific information. For example, the EKS AMI family bootstrap process:

1. Karpenter injects cluster endpoint, CA bundle, kubelet configuration, DNS settings, and other NodePool-specified values
2. Creates a complete `/etc/eks/bootstrap.sh` command with all necessary parameters
3. Combines the generated bootstrap script with any user-provided custom userData using MIME multipart format

The EKS bootstrap implementation within karpenter-provider-aws automatically provides:

- Cluster name and endpoint
- Certificate authority bundle
- Kubelet configuration and node registration parameters derived from NodePool labels, NodeClaim taints, and EC2NodeClass kubelet settings
- DNS cluster IP configuration
- IPv6 support detection
- Instance store configuration

In contrast, the current Custom AMI family implementation is minimal:

```go
func (e Custom) Script() (string, error) {
    return base64.StdEncoding.EncodeToString([]byte(aws.ToString(e.Options.CustomUserData))), nil
}
```

Custom AMI users receive **only** their raw `userData` with no automatic injection of cluster context. This means they must either hardcode all bootstrapping information (cluster endpoints, kubelet config, etc.) directly in the `userData` field, or build AMIs that can retrieve this information independently during boot.

### Drift Detection

Karpenter's drift feature ensures nodes conform to their desired state by comparing running instances' launch templates against what would be generated from the current `EC2NodeClass` spec. Any change to `spec.userData` triggers drift, marking all associated nodes for replacement.

While this is correct for infrastructure changes, it creates unnecessary churn when only dynamic data (like rotating credentials) has been updated without any change to the underlying bootstrap logic.

## Not in Scope

- This document is not proposing to fully implement automatic bootstrapping or all the features that standard AMI families provide.
- This document is not proposing to be able to toggle on/off the static drift capability for user-defined fields in the `EC2NodeClass` spec.

## Solutions

### Option 1: Variable Templating

This option introduces Go template-style variable templating that supports both Karpenter-managed values and user-defined variables. Karpenter-managed values (cluster endpoint, labels, etc.) are automatically available and trigger drift when changed. User-defined variables can be configured to either trigger drift or be ignored during drift calculation.

```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: default
spec:
  amiFamily: Custom
  userDataVariables:
    - name: "AUTH_TOKEN"
      value: "rotated-secret-token-value"
      drift: false
    - name: "CONFIG_VERSION"
      value: "v1.2.3"
      drift: true
  userData: |
    {
      "ignition": {
        "config": {
          "merge": [
            {
              "httpHeaders": [
                              {
                "name": "Authorization",
                "value": "Bearer {{.AUTH_TOKEN}}"
              },
              {
                "name": "X-Cluster",
                "value": "{{.ClusterName}}"
              }
            ],
            "source": "https://{{.ClusterEndpoint}}/bootstrap/{{.CONFIG_VERSION}}"
            }
          ]
        },
      }
    }
```

#### Implementation Details

Karpenter will need to:

1. Automatically populate a set of known variables that are used as node bootstrapping paramaters. In other words, the fields in the [Options](https://github.com/aws/karpenter-provider-aws/blob/a843836fe85b8e0f82875beacffb68ad226a32a7/pkg/providers/amifamily/bootstrap/bootstrap.go#L30) struct would be available as variables in the `userData` template.

    Example:
   - ClusterName as `{{.ClusterName}}`
   - ClusterEndpoint as `{{.ClusterEndpoint}}`
   - Taints as `{{.Taints}}`
   - Labels as `{{.Labels}}`
   - KubeletConfig as `{{.KubeletConfig}}`

2. Parse `userData` template and substitute Go template variables from the well-known variables as well as `userDataVariables`
3. Use Go's `text/template` package with `{{.Variable}}` syntax for variable substitution and pass the end result as the new `userData` for the launch template.
4. Ignore triggering drift on changes to `userDataVariables` entries where `drift` is set to `false`.

For simplicity, the output of each template variable will be stringified, as defined by the Go template engine. It is up to the AMI bootstrapper itself to handle the resulting `userData` format.

#### Advantages

- Reduces configuration duplication when using Custom AMI
- Uses well-known `{{.Variable}}` templating from Go's text/template package
- Works with any userData format and AMIFamily

#### Disadvantages

- Requires coupling between the API and Karpenter's Options struct
- Requires API implementations to be aware of the output format of the well-known variables

### Option 2: HTTP-based userData Resolution

This option introduces a mechanism where `userData` is fetched from an external HTTP endpoint at launch template creation time. Instead of embedding userData directly in the `EC2NodeClass`, the controller makes an HTTP request to resolve the content dynamically. The HTTP server implementation is responsible for managing node bootstrapping data and can provide both Karpenter-managed values and dynamic configuration.

```yaml
apiVersion: karpenter.k8s.aws/v1beta1
kind: EC2NodeClass
metadata:
  name: default
spec:
  # HTTP endpoint configuration for userData resolution
  userDataResolver:
    url: "https://bootstrap.example.com/userdata/v1"
    method: GET  # Optional, defaults to GET
    headers:
      - name: "X-Cluster-ID"
        value: "my-production-cluster"
      - name: "X-Node-Class"
        value: "default"
      - name: "Authorization"
        valueFrom:
          secretRef:
            name: bootstrap-auth
            key: token
    timeout: 30s  # Optional
    # Only the resolver configuration affects drift, not the resolved content
```

#### Implementation Details

1. When creating a launch template, the Karpenter controller makes an HTTP request to the specified URL with the configured headers
2. The HTTP response body becomes the `userData` for the launch template
3. Drift calculation considers only the `userDataResolver` configuration (URL, headers, etc.), not the actual resolved content
4. Headers can be static values or references to Kubernetes Secrets for sensitive data
5. The controller includes reasonable defaults for timeouts and retry behavior

#### Advantages

- userData content can be completely external to the EC2NodeClass
- userData logic can be centralized in external services
- Only resolver configuration changes trigger drift, not the resolved content

#### Disadvantages

- Introduces dependency on external HTTP service availability
- Requires network connectivity from Karpenter controller to the HTTP service
- HTTP requests add latency to launch template creation
- Users must implement and maintain the HTTP service that provides appropriate node bootstrapping data

### Note about Go templating

Variable templating uses Go's `text/template` syntax with double curly braces and dot notation (e.g., `{{.AUTH_TOKEN}}`). The controller performs template rendering using Go's standard `text/template` package. To include literal `{{}}` in userData, escape them using Go template escaping or raw strings.

Note that it is possible for this feature to be used to inject arbitrary shell scripts into the `userData` field. However, the `userData` field itself is already capable of running arbitrary shell scripts, so this addition is not a new vector. It should be noted that the user who is in control of the `userData` field should also be in control of the `userDataVariables` field to mitigate this risk.

## Recommendation

We recommend **Option 1: Variable Templating** as the initial implementation. It solves the use cases outlined in this RFC and is a simple implementation that can be extended in the future.

## Common Gotchas

- **Does your change introduce new APIs?**  
  Yes. This RFC proposes adding `spec.userDataVariables` to the `EC2NodeClass` API. This feature would be considered beta, aligned with the stability of the CRD. The Go template syntax (`{{.Variable}}`) also becomes a supported part of the API and must be clearly documented.

- **Does your change behave differently with different cloud providers?**  
  No. This change is confined to the `EC2NodeClass`, which is specific to the AWS provider. It does not affect any vendor-neutral interfaces.

- **Does your change expose details users may rely on?**  
  Yes, the Go templating syntax and the `userDataVariables` field become part of the public interface. Users may build automation around this feature, so it must be treated as a stable contract once implemented.

- **Does your change impact performance?**  
  The performance impact is expected to be negligible. The variable substitution is a simple in-memory string operation that occurs during the Karpenter's launch template creation, which happens when creating new `NodeClaims`.
