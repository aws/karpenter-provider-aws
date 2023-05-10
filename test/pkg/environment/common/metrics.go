package common

import (
	"fmt"
	"strings"
	"time"

	//nolint:revive,stylecheck
	. "github.com/onsi/gomega" //nolint:revive,stylecheck
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/samber/lo"
)

const (
	machinesCreated              = "karpenter_machines_created"
	nodesCreated                 = "karpenter_nodes_created"
	podStartupTime               = "karpenter_pods_startup_time_seconds"
	nodeTerminationTime          = "karpenter_nodes_termination_time_seconds"
	schedulingDuration           = "karpenter_provisioner_scheduling_duration_seconds_bucket"
	deprovisioningDurationBucket = "karpenter_deprovisioning_evaluation_duration_seconds_bucket"
	cloudProviderMethodDuration  = "karpenter_cloudprovider_duration_seconds_bucket"
	cloudProviderErrorsTotal     = "karpenter_cloudprovider_errors_total"
)

func (env *Environment) ExpectPrometheusQuery(metric string, labels map[string]string) model.Vector {
	karpenterPod := env.ExpectActiveKarpenterPodWithOffset(1)

	labels = lo.Assign(labels, map[string]string{"pod": karpenterPod.Name, "namespace": karpenterPod.Namespace})
	value, warn, err := env.PromClient.Query(env.Context, buildQueryString(metric, labels), time.Now())
	Expect(warn).To(HaveLen(0))
	Expect(err).To(BeNil())
	return value.(model.Vector)
}

func (env *Environment) ExpectPrometheusSummaryQuantile(metric string, quantile float64, labels map[string]string) model.Vector {
	karpenterPod := env.ExpectActiveKarpenterPodWithOffset(1)

	labels = lo.Assign(labels, map[string]string{"pod": karpenterPod.Name, "namespace": karpenterPod.Namespace, "quantile": fmt.Sprint(quantile)})
	value, warn, err := env.PromClient.Query(env.Context, buildQueryString(metric, labels), time.Now())
	Expect(warn).To(HaveLen(0))
	Expect(err).To(BeNil())
	return value.(model.Vector)
}

func (env *Environment) ExpectPrometheusHistogramQuantile(metric string, quantile float64, labels map[string]string) model.Vector {
	karpenterPod := env.ExpectActiveKarpenterPodWithOffset(1)

	labels = lo.Assign(labels, map[string]string{"pod": karpenterPod.Name, "namespace": karpenterPod.Namespace})
	value, warn, err := env.PromClient.Query(env.Context, buildHistogramQuantileQueryString(metric, quantile, labels), time.Now())
	Expect(warn).To(HaveLen(0))
	Expect(err).To(BeNil())
	return value.(model.Vector)
}

func (env *Environment) ExpectPrometheusRangeQuery(metric string, labels map[string]string, r promv1.Range) model.Vector {
	karpenterPod := env.ExpectActiveKarpenterPodWithOffset(1)

	labels = lo.Assign(labels, map[string]string{"pod": karpenterPod.Name, "namespace": karpenterPod.Namespace})
	value, warn, err := env.PromClient.QueryRange(env.Context, buildQueryString(metric, labels), r)
	Expect(warn).To(HaveLen(0))
	Expect(err).To(BeNil())
	return value.(model.Vector)
}

// ExpectSLOsMaintained describes a set of expectations that Karpenter MUST meet in order to ensure its SLOs
func (env *Environment) ExpectSLOsMaintained() {
	fmt.Printf("# of CloudProvider Create Errors: %s\n", env.ExpectPrometheusQuery(cloudProviderErrorsTotal, map[string]string{"method": "Create"}))

	fmt.Printf("P99 for CloudProvider Create: %s\n", env.ExpectPrometheusHistogramQuantile(cloudProviderMethodDuration, 0.99, map[string]string{"method": "Create"}))
	fmt.Printf("P90 for CloudProvider Create: %s\n", env.ExpectPrometheusHistogramQuantile(cloudProviderMethodDuration, 0.90, map[string]string{"method": "Create"}))
	fmt.Printf("P50 for CloudProvider Create: %s\n", env.ExpectPrometheusHistogramQuantile(cloudProviderMethodDuration, 0.50, map[string]string{"method": "Create"}))

	fmt.Printf("P99 for Scheduling: %s\n", env.ExpectPrometheusHistogramQuantile(schedulingDuration, 0.99, nil))
	fmt.Printf("P90 for Scheduling: %s\n", env.ExpectPrometheusHistogramQuantile(schedulingDuration, 0.90, nil))
	fmt.Printf("P50 for Scheduling: %s\n", env.ExpectPrometheusHistogramQuantile(schedulingDuration, 0.50, nil))

	fmt.Printf("P99 for Deprovisioning Emptiness: %s\n", env.ExpectPrometheusHistogramQuantile(deprovisioningDurationBucket, 0.99, map[string]string{"method": "emptiness"}))
	fmt.Printf("P90 for Deprovisioning Emptiness: %s\n", env.ExpectPrometheusHistogramQuantile(deprovisioningDurationBucket, 0.90, map[string]string{"method": "emptiness"}))
	fmt.Printf("P50 for Deprovisioning Emptiness: %s\n", env.ExpectPrometheusHistogramQuantile(deprovisioningDurationBucket, 0.50, map[string]string{"method": "emptiness"}))

	fmt.Printf("P99 for Pod Startup Time: %s\n", env.ExpectPrometheusSummaryQuantile(podStartupTime, 0.99, nil))
	fmt.Printf("P90 for Pod Startup Time: %s\n", env.ExpectPrometheusSummaryQuantile(podStartupTime, 0.9, nil))
	fmt.Printf("P90 for Pod Startup Time: %s\n", env.ExpectPrometheusSummaryQuantile(podStartupTime, 0.5, nil))

	fmt.Printf("P99 for Node Termination Time: %s\n", env.ExpectPrometheusSummaryQuantile(nodeTerminationTime, 0.99, nil))
	fmt.Printf("P90 for Node Termination Time: %s\n", env.ExpectPrometheusSummaryQuantile(nodeTerminationTime, 0.9, nil))
	fmt.Printf("P90 for Node Termination Time: %s\n", env.ExpectPrometheusSummaryQuantile(nodeTerminationTime, 0.5, nil))
}

func buildHistogramQuantileQueryString(metric string, quantile float64, labels map[string]string) string {
	var labelStr string
	if len(labels) > 0 {
		labelStr = fmt.Sprintf("{%s}", strings.Join(lo.MapToSlice(labels, func(k, v string) string { return fmt.Sprintf(`%s="%s"`, k, v) }), ","))
	}
	return fmt.Sprintf(`histogram_quantile(%f, sum(rate(%s%s[5m])) by (le))`, quantile, metric, labelStr)
}

func buildQueryString(metric string, labels map[string]string) string {
	var labelStr string
	if len(labels) > 0 {
		labelStr = fmt.Sprintf("{%s}", strings.Join(lo.MapToSlice(labels, func(k, v string) string { return fmt.Sprintf(`%s="%s"`, k, v) }), ","))
	}
	return fmt.Sprintf(`%s%s`, metric, labelStr)
}
