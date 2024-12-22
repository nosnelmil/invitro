package common

const (
	TraceFormatString = "{}"
)

// Multi-loader possible collectable metrics
const (
	Activator  string = "activator"
	AutoScaler string = "autoscaler"
	TOP        string = "top"
	Prometheus string = "prometheus"
)

var ValidCollectableMetrics = []string{Activator, AutoScaler, TOP, Prometheus}
