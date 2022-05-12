
---
title: "Getting Started with kOps"
linkTitle: "Getting Started with kOps"
weight: 10
description: >
  Set up Karpenter with a kOps cluster
---

In this example, the cluster will be running on Amazon Web Services (AWS) managed by [kOps](https://kops.sigs.k8s.io/).
Karpenter is designed to be cloud provider agnostic, but currently only supports AWS. Contributions are welcomed

Karpenter is supported on kOps as of 1.24.0-alpha.2, but sits behind a feature flag as the interface between kOps and Karpenter is
still work in progress and is likely to change significantly. This guide is intended for users that wants to test Karpenter on kOps and provide feedback to Karpenter and kOps developers.
Read more about how Karpenter works on kOps and the current limitations in the [kOPs Karpenter documentation](https://kops.sigs.k8s.io/operations/karpenter/).

This guide should take less than 1 hour to complete, and cost less than $0.25.
Follow the clean-up instructions to reduce any charges.

This guide assumes you already have a kOps state store and a hosted zone. If you do not have one,
run through the [kOps getting started on AWS documentation](https://kops.sigs.k8s.io/getting_started/aws/) up until "Creating your first cluster".

## Install

Karpenter is installed in clusters as a managed addon. kOps will automatically create 
and manage the necessary the IAM roles and policies Karpenter needs.

### Required Utilities

Install these tools before proceeding:

1. `kubectl` - [the Kubernetes CLI](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/)
2. `kops` - [kubectl, but for clusters](https://kops.sigs.k8s.io/getting_started/install/) v1.24.0 or later

### Environment Variables

After setting up the tools, set the following environment variables used by kOps.

```bash
export KOPS_FEATURE_FLAGS=Karpenter
export CLUSTER_NAME=${USER}-karpenter-demo.example.com
export ZONES=us-west-2a
export KOPS_STATE_STORE=s3://prefix-example-com-state-store
export KOPS_OIDC_STORE=s3://prefix-example-com-oidc-store/discovery
```

### Create a Cluster

kOps installs Karpenter on the control plane. Once the control plane is running, Karpenter will provision the
the worker nodes needed for non-Control Plane Deployments such as CoreDNS and CSI drivers.

The following command will launch a cluster with Karpenter-managed worker nodes:

```bash
kops create cluster \
    --zones=$ZONES \
    --discovery-store=${KOPS_OIDC_STORE} \
    --instance-manager=karpenter \
    --networking=amazonvpc \
    ${CLUSTER_NAME} \
    --yes
```

Note: we are using AWS VPC CNI for networking as Karpenter's binpacking logic assumes ENI-based networking.

### Provisioner

A single Karpenter provisioner is capable of handling many different pod
shapes. Karpenter makes scheduling and provisioning decisions based on pod
attributes such as labels and affinity. In other words, Karpenter eliminates
the need to manage many different InstanceGroups.

kOps manage provisioners through InstanceGroups. Your cluster will already have
one Provisioner that will contain a suitable set of instance types for Karpenter to
choose from.

Managing Provisioner resources directly is possible, but not straight-forward. Read
more about managing provisioners in the [kOPs Karpenter documentation](https://kops.sigs.k8s.io/operations/karpenter/)

## First Use

Karpenter is now active and ready to begin provisioning nodes.
As mentioned above, you should already have some Karpenter-managed nodes in your cluster used by
other kOps addons. Create additional pods using a Deployment, and watch Karpenter provision nodes in response.

### Automatic Node Provisioning

This deployment uses the [pause image](https://www.ianlewis.org/en/almighty-pause-container) and starts with zero replicas.

```bash
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inflate
spec:
  replicas: 0
  selector:
    matchLabels:
      app: inflate
  template:
    metadata:
      labels:
        app: inflate
    spec:
      terminationGracePeriodSeconds: 0
      containers:
        - name: inflate
          image: public.ecr.aws/eks-distro/kubernetes/pause:3.2
          resources:
            requests:
              cpu: 1
EOF
kubectl scale deployment inflate --replicas 5
kubectl logs -f -n kube-system -l app.kubernetes.io/name=karpenter -c controller
```

### Automatic Node Termination

Now, delete the deployment. After 30 seconds,
Karpenter should terminate the now empty nodes.

```bash
kubectl delete deployment inflate
kubectl logs -f -n karpenter -l app.kubernetes.io/name=karpenter -c controller
```

### Manual Node Termination

If you delete a node with kubectl, Karpenter will gracefully cordon, drain,
and shutdown the corresponding instance. Under the hood, Karpenter adds a
finalizer to the node object, which blocks deletion until all pods are
drained and the instance is terminated. Keep in mind, this only works for
nodes provisioned by Karpenter.

```bash
kubectl delete node $NODE_NAME
```

This is similar to `kops delete instance $NODE_NAME` except for that kOps will not respect
`karpenter.sh/do-not-evict`.

## Upgrading a Cluster

kOps is aware of nodes managed by Karpenter and will handle [rolling upgrades](https://kops.sigs.k8s.io/operations/rolling-update/) of those nodes the same way as any other node:

```
kops upgrade cluster --yes
kops update cluster --yes
kops rolling-update cluster --yes
```

Karpenter-managed InstanceGroups supports setting `maxUnavailable`, but since Karpenter instances do not run in an Auto Scaling Group, setting `maxSurge` will not have any effect.

During rolling updates, `karpenter.sh/do-not-evict` is not respected.

## Cleanup

To avoid additional charges, remove the demo infrastructure from your AWS account.

```bash
kops delete cluster $CLUSTER_NAME --yes
```
