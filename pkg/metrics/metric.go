package metric

// Metric encapsulates
type Metric interface {
	// TargetValue returns a statically configured value for the metric
	Target() float64
	// ObservedValue returns a dynamically observed value for the metric
	Observed() float64
}
