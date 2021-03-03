# Bin packing

Karpenter provisions instances based on the number of pending pods and their resource requirements (CPU and Memory). These requirements vary and is configured by the pod owners. Karpenter needs to make sure that there is sufficient capacity among the provisioned instances for these pods to be scheduled.

The easiest way is to provision an instance per pending pod, however, its not very efficient and can incur infrastructure cost for the user. In order to be able to schedule pods efficiently and be cost effective, Karpenter packs these pods on to the available instance types with the given set of constraints. Karpenter follows the [First Fit Decreasing](https://en.wikipedia.org/wiki/Bin_packing_problem#First_Fit_Decreasing_(FFD)) algorithm for bin packing the pods.

At a very high level, filter and group the pending pods into these smaller groups which can be scheduled together in one or more nodes in the same zone. Once these pods are grouped, following steps are taken to bin pack each group individually -

- Sort the pods in the group based on non-increasing order of the resources requested.
- Filter all the instance types available, given the cloud provider and node contraints (such as - zones, architecture).
- Sort these instance based on increasing order of their capacity (CPU and memory).
- Loop through the pods, starting with the biggest pod and selecting the smallest instance type.
    - If the pod doesn't fit this instance type, skip and select next bigger instance type.
    - If the pod fits this instance type, check how many more pods we can fit on this instance type.
    - Compare all the instance types and select the one with max pods fit.
- Select the next biggest pod which is not packed in previous iterations and follow the same procedure starting with smallest instance type.

