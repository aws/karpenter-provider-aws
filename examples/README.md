## Examples

Checkout example Provisioner and workload specs to demo with Karpenter.

## Usage:

Provisioner specs expect a `CLUSTER_NAME` environment variable to be set to your cluster name. You can use the following command to substitute the environment variable and `kubectl apply` to your cluster:

```
CLUSTER_NAME=<my-cluster-name> envsubst < provisioner/spot.yaml | kubectl apply -f -
```
