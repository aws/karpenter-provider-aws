# Roadmap
## Q1 2021
### [Feature] Pending Pods Alpha (@prateekgogia)
Enable users to scale node groups based off of pods that are unable to schedule due to resource constraints. This approach is similar to the cluster autoscaler. We expect this feature to be experimental or of "alpha" quality.

### [Feature] Scheduled Autoscaling (@njtran)
Enable users to scale based off of a predefined schedule.

## Q2 2021
### [Feature] Pending Pods (@prateekgogia)
General availability of the Pending Pods feature.

## Q2 2021
### [Performance] PID Controller for HorizontalAutoscaler
Explore [PID](https://en.wikipedia.org/wiki/PID_controller) as an alternative to proportional autoscaling for Karpenter's core algorithm.

## Q3 2021
### [Scalability] Karpenter Scale Testing
Scale testing for all of Karpenter's metrics producers. Explore 10,000 nodes, 100s of scalers, etc.
