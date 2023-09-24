# kbump

Kbump is a simple CLI tool to bump the API manifests from `v1alpha5` to `v1beta1`.
It converts `Provisioner` to `NodePool` and `AWSNodeTemplate` to `EC2NodeClass`.

## Installation 

```
go install github.com/aws/karpenter/tools/kbump/cmd/kbump
```

## Usage:

```
cat provisioner.yaml | kbump | > nodepool.yaml
cat nodetemplate.yaml | kbump -r MyAwsRole | > nodeclass.yaml
```