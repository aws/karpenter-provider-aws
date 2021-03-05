# Bin packing

Karpenter provisions instances based on the number of pending pods and their resource requirements (CPU and Memory). These requirements vary and are configured by the pod owners. Karpenter needs to make sure that there is sufficient capacity among the provisioned instances for these pods to be scheduled.

The easiest way is to provision an instance per pending pod, however, this is not very efficient and can incur infrastructure costs for the user. In order to be able to schedule pods efficiently and be cost effective, Karpenter packs these pods on to the available instance types with the given set of constraints. Karpenter follows the [First Fit Decreasing (FFD)](https://en.wikipedia.org/wiki/Bin_packing_problem#First_Fit_Decreasing_(FFD)) algorithm for bin packing the pods. The FFD technique chosen is less complex and relatively faster considering the number of pods and instance types available are not too high.

At a very high level, filter and group the pending pods into these smaller groups which can be scheduled together in one or more nodes in the same zone. Once these pods are grouped, following steps are taken to bin pack each group individually -

1. Sort the pods in the group based on non-increasing order of the resources requested.
2. Filter all the instance types available, given the cloud provider and node contraints (such as - zones, architecture).
3. Start with the largest pod and an instance type.
    - If the pod doesn't fit this instance type, skip this instance type and select next bigger instance type.
    - If the pod fits this instance type, check all the remaining pods from largest to smallest how many can fit on this given instance type. 
    - Compare all the instance types starting with the largest pod in step 3 and select the instance type with max pods fit.
4. Loop through the remaining pods, any pod which is not packed in previous iterations, follow the same procedure in step 3.
