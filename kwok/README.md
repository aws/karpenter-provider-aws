# AWS KWOK Provider

Before using the aws kwok provider, make sure that you don't have an installed version of Karpenter in your cluster.

## Requirements
- Have an image repository that you can build, push, and pull images from.
    - For an example on how to set up an image repository refer to [karpenter.sh](https://karpenter.sh/docs/contributing/development-guide/#environment-specific-setup)
- Have a cluster that you can install Karpenter on to.
    - For an example on how to make a cluster in AWS, refer to [karpenter.sh](https://karpenter.sh/docs/getting-started/getting-started-with-karpenter/)

If you use a kind cluster, please set the the following environment variables:
```bash
export KO_DOCKER_REPO=kind.local
export KIND_CLUSTER_NAME=<kind cluster name, for example, chart-testing>
```

## Installing
```bash
make apply-kwok
make apply # Run this command again to redeploy if the code has changed
```

## Create a NodePool

Once kwok is installed and Karpenter successfully applies to the cluster, you should now be able to create a NodePool.

```bash
export CLUSTER_NAME=<cluster-name>

cat <<EOF | envsubst | kubectl apply -f -
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: default
spec:
  template:
    spec:
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]
        - key: kubernetes.io/os
          operator: In
          values: ["linux"]
        - key: karpenter.sh/capacity-type
          operator: In
          values: ["on-demand"]
        - key: karpenter.k8s.aws/instance-category
          operator: In
          values: ["c", "m", "r"]
        - key: karpenter.k8s.aws/instance-generation
          operator: Gt
          values: ["2"]
      nodeClassRef:
        group: karpenter.k8s.aws
        kind: EC2NodeClass
        name: default
      expireAfter: 720h # 30 * 24h = 720h
  limits:
    cpu: 1000
  disruption:
    consolidationPolicy: WhenEmptyOrUnderutilized
    consolidateAfter: 1m
---
apiVersion: karpenter.k8s.aws/v1
kind: EC2NodeClass
metadata:
  name: default
spec:
  role: "KarpenterNodeRole-${CLUSTER_NAME}" # replace with your cluster name
  amiSelectorTerms:
    - alias: "al2023@latest"
  subnetSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}" # replace with your cluster name
  securityGroupSelectorTerms:
    - tags:
        karpenter.sh/discovery: "${CLUSTER_NAME}" # replace with your cluster name
EOF
```

## Taint the existing nodes

```bash
kubectl taint nodes <existing node name> CriticalAddonsOnly:NoSchedule
```
After doing this, you can create a deployment to test node scaling with kwok provider.

## Uninstalling
```bash
make delete
make delete-kwok
```
