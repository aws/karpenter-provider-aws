package metrics

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"karpenter-benchmark/utils/portforward"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// LookupMetrics fetches all metrics from the specified URL and returns them as a map
func LookupMetrics(metricsURL string) (map[string]*dto.MetricFamily, error) {
	if err := portforward.DefaultPortForward(); err != nil {
		log.Printf("Warning: Could not setup port forwarding: %v", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	log.Printf("Fetching metrics from %s", metricsURL)

	// Send GET request to the metrics endpoint
	resp, err := fetchWithRetry(client, "http://"+metricsURL, 5)
	// Check specific error types
	if err != nil {
		return nil, fmt.Errorf("error fetching metrics: %v", err)
	}
	defer resp.Body.Close()

	// Parse metrics
	parser := expfmt.TextParser{}
	metrics, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing metrics: %v", err)
	}

	return metrics, nil
}

// PrintAllMetrics fetches and prints metrics from the specified URL
func PrintMetrics(metricsURL string) error {
	metrics, err := LookupMetrics(metricsURL)
	if err != nil {
		return err
	}

	// Process and print all available metrics
	fmt.Println("\nMetrics:")
	fmt.Println("======================")

	// Count the total number of metrics
	metricCount := 0

	// Iterate through all metrics
	for name, mf := range metrics {
		if name != "karpenter_scheduler_scheduling_duration_seconds" &&
			name != "karpenter_voluntary_disruption_decision_evaluation_duration_seconds" &&
			name != "container_cpu_cfs_throttled_seconds_total" &&
			name != "container_cpu_load_average_10s" &&
			name != "container_cpu_usage_seconds_total" &&
			name != "container_memory_usage_bytes" &&
			name != "container_memory_working_set_bytes" &&
			name != "process_cpu_seconds_total" &&
			name != "go_memstats_alloc_bytes_total" {
			continue
		}
		metricCount++
		fmt.Printf("\n%s (%s)\n", mf.GetName(), mf.GetHelp())

		// Print each metric instance with its labels and value
		for _, m := range mf.GetMetric() {
			labels := ""
			for _, l := range m.GetLabel() {
				labels += fmt.Sprintf("%s=%s ", l.GetName(), l.GetValue())
			}

			var value float64
			var valueType string

			if m.GetCounter() != nil {
				value = m.GetCounter().GetValue()
				valueType = "counter"
			} else if m.GetGauge() != nil {
				value = m.GetGauge().GetValue()
				valueType = "gauge"
			} else if m.GetHistogram() != nil {
				// convert to quantile
				hist := m.GetHistogram()
				quantileP50 := calculateHistogramQuantile(0.50, hist)
				quantileP90 := calculateHistogramQuantile(0.90, hist)
				quantileP99 := calculateHistogramQuantile(0.99, hist)
				value = hist.GetSampleSum()
				valueType = fmt.Sprintf("histogram (p50: %.3f, p90: %.3f, p99: %.3f)", quantileP50, quantileP90, quantileP99)
			} else if m.GetSummary() != nil {
				value = m.GetSummary().GetSampleSum()
				valueType = "summary"
			} else {
				valueType = "unknown"
			}

			fmt.Printf("  %s[%s]= %v\n", labels, valueType, value)
		}
	}

	fmt.Printf("\nTotal metrics found: %d\n", metricCount)
	return nil
}

func fetchWithRetry(client *http.Client, url string, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	var err error

	// Initial delay (will be increased with each retry)
	delay := 100 * time.Millisecond

	// Try the request up to maxRetries times
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			// Exponential backoff with jitter
			delay = time.Duration(float64(delay) * 1.5)
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}
		}

		// Create a new request for each attempt
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %v", err)
		}

		// Execute the request
		resp, err = client.Do(req)

		// Check for specific error types
		if err != nil {
			// Handle specific error types
			if netErr, ok := err.(net.Error); ok {
				if netErr.Timeout() {
					log.Printf("Timeout error: %v", err)
					continue // Retry on timeout
				}
			}

			// Check for EOF errors
			if strings.Contains(err.Error(), "EOF") || err == io.EOF {
				log.Printf("EOF error: %v", err)
				continue // Retry on EOF
			}

			// Check for connection reset
			if strings.Contains(err.Error(), "connection reset") {
				log.Printf("Connection reset: %v", err)
				continue // Retry on connection reset
			}

			// For other errors, log and retry
			log.Printf("Error during request: %v", err)
			continue
		}

		// If we got here, the request was successful
		return resp, nil
	}

	// If we exhausted all retries, return the last error
	return nil, fmt.Errorf("failed after %d retries: %v", maxRetries, err)
}

// calculateHistogramQuantile calculates the specified quantile from a Prometheus histogram
func calculateHistogramQuantile(q float64, h *dto.Histogram) float64 {
	if h == nil || len(h.Bucket) == 0 {
		return 0
	}

	// Extract buckets and counts
	var buckets []float64
	var counts []uint64
	var totalCount uint64

	for _, b := range h.Bucket {
		buckets = append(buckets, b.GetUpperBound())
		counts = append(counts, b.GetCumulativeCount())
		totalCount = b.GetCumulativeCount()
	}

	if totalCount == 0 {
		return 0
	}

	// Find the target count for the quantile
	targetCount := uint64(float64(totalCount) * q)

	// Find the bucket that contains the quantile
	for i, count := range counts {
		if count >= targetCount {
			// If this is the first bucket, return its upper bound
			if i == 0 {
				return buckets[i]
			}

			// Linear interpolation within the bucket
			lowerBound := buckets[i-1]
			upperBound := buckets[i]
			lowerCount := counts[i-1]
			upperCount := counts[i]

			// If bucket counts are the same, return the average
			if upperCount == lowerCount {
				return (upperBound + lowerBound) / 2
			}

			// Linear interpolation
			ratio := float64(targetCount-lowerCount) / float64(upperCount-lowerCount)
			return lowerBound + ratio*(upperBound-lowerBound)
		}
	}

	// If we get here, the quantile is in the +Inf bucket
	if len(buckets) > 0 {
		return buckets[len(buckets)-1]
	}

	return 0
}
