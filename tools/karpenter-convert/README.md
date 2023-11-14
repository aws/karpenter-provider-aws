# karpenter-convert

karpenter-convert is a simple CLI tool to bump the API manifests from alpha to beta.
It converts `v1alpha5/Provisioner` to `v1beta1/NodePool` and `v1alpha1/AWSNodeTemplate` to `v1beta1/EC2NodeClass`.

## Installation 

```
go install github.com/aws/karpenter/tools/karpenter-convert/cmd/karpenter-convert@latest
```
*NOTE:  requires go 1.21+*

## Usage
```console
Usage:
  karpenter-convert [flags]

Flags:
  -f, --filename strings               Filename, directory, or URL to files to need to get converted.
  -h, --help                           help for karpenter-convert
      --ignore-defaults                Ignore defining default requirements when migrating Provisioners to NodePool.
  -o, --output string                  Output format. One of: (json, yaml, name, go-template, go-template-file, template, templatefile, jsonpath, jsonpath-as-json, jsonpath-file). (default "yaml")
  -R, --recursive                      Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
```

## Examples:

```console
# Convert a single Provisioner file to NodePool
karpenter-convert -f provisioner.yaml > nodepool.yaml

# Convert a single AWSNodeTemplate file to EC2NodeClass
karpenter-convert -f nodetemplate.yaml > nodeclass.yaml

# Convert an entire directory (.json, .yaml, .yml files) to the equivalent new APIs
karpenter-convert -f . > output.yaml

# Convert a single provisioner and apply directly to the cluster
karpenter-convert -f provisioner.yaml | kubectl apply -f -

# Convert a single provisioner ignoring the default requirements
# Karpenter provisioners had logic to default Instance Families, OS, Architecture and Cpacity type when these were not provided.
# NodePool drops these defaults, and you can avoid that the conversion tools applies them for you during the conversion
karpenter-convert --ignore-defaults -f provisioner.yaml > nodepool.yaml
```

## Usage notes

When converting an AWSNodeTemplate to EC2NodeClass, the newly introduced field `role` can't be mapped automatically.
The tool leaves a placeholder `$KARPENTER_NODE_ROLE` which needs to be manually updated.
