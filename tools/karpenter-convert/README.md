# karpenter-convert

karpenter-convert is a simple CLI tool to bump the API manifests from alpha to beta.
It converts `v1alpha5/Provisioner` to `v1beta1/NodePool` and `v1alpha1/AWSNodeTemplate` to `v1beta1/EC2NodeClass`.

## Installation 

```
go install github.com/aws/karpenter/tools/karpenter-convert/cmd/karpenter-convert@latest
```

## Usage:

```console
# Convert a single Provisioner file to NodePool
karpenter-convert -f provisioner.yaml > nodepool.yaml

# Convert a single AWSNodeTemplate file to EC2NodeClass
karpenter-convert -f nodetemplate.yaml > nodeclass.yaml

# Convert an entire directory (.json, .yaml, .yml files) to the equivalent new APIs
karpenter-convert -f . > output.yaml

# Convert a single provisioner and apply directly to the cluster
karpenter-convert -f provisioner.yaml | kubectl apply -f -
```

## Usage notes

When converting an AWSNodeTemplate to EC2NodeClass, the newly introduced field `role` can't be mapped automatically.
The tool leaves a placeholder `$KARPENTER_NODE_ROLE` which needs to be manually updated.
