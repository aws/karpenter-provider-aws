package v1beta1

// Karpenter aws specific taints
var (
	InterruptionTaintKey                          = Group + "/interruption"
	InterruptionRebalanceRecommendationTaintValue = "RebalanceRecommendation"
	InterruptionScheduledChangeTaintValue         = "ScheduledChange"
	InterruptionSpotInterruptionTaintValue        = "SpotInterruption"
	InterruptionStateChangeTaintValue             = "StateChange"
)
