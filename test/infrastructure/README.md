# Infrastructure

Run `./setup-management-cluster.sh` to create an EKS cluster with the following add-ons installed:
- [Tekton](https://tekton.dev/)
- [AWS Load Balancer](https://github.com/kubernetes-sigs/aws-load-balancer-controller)
- [EBS CSI Driver](https://github.com/kubernetes-sigs/aws-ebs-csi-driver)
- `Karpenter`
- [Prometheus](https://prometheus.io/)
- [KIT Operator](https://github.com/awslabs/kubernetes-iteration-toolkit/tree/main/operator)

More information about the design choices will be coming. Refer to the /test/README.md for more.
