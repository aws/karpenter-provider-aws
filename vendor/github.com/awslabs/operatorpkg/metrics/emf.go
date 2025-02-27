package metrics

import (
	"encoding/json"
	"io"
	"time"

	"github.com/samber/lo"
)

type EMF struct {
	writer               io.Writer
	namespace            string
	name                 string
	dimensions           [][]string
	additionalProperties []map[string]string
}

func NewEMF(writer io.Writer, namespace, name string, dimensions [][]string, additionalProperties ...map[string]string) *EMF {
	return &EMF{writer: writer, namespace: namespace, name: name, dimensions: dimensions, additionalProperties: additionalProperties}
}

func (e *EMF) withDimensions(entry Entry, dimensions ...[]string) Entry {
	entry.AddDimensions(dimensions...)
	return entry
}

func (e *EMF) withProperties(entry Entry, labels map[string]string) Entry {
	for k, v := range labels {
		entry.AddProperty(k, v)
	}
	for _, m := range e.additionalProperties {
		for k, v := range m {
			entry.AddProperty(k, v)
		}
	}
	return entry
}

type EMFCounter struct {
	*EMF
}

func NewEMFCounter(writer io.Writer, namespace, name string, dimensions [][]string, additionalProperties ...map[string]string) CounterMetric {
	return &EMFCounter{EMF: NewEMF(writer, namespace, name, dimensions, additionalProperties...)}
}

func (e *EMFCounter) Inc(labels map[string]string) {
	entry := e.withDimensions(e.withProperties(NewEntry(e.namespace), labels), e.dimensions...)
	entry.AddMetric(e.name, 1)
	lo.Must(e.writer.Write([]byte(lo.Must(entry.Build()) + "\n")))
}

func (e *EMFCounter) Add(v float64, labels map[string]string) {
	entry := e.withProperties(NewEntry(e.namespace), labels)
	entry.AddMetric(e.name, v)
	lo.Must(e.writer.Write([]byte(lo.Must(entry.Build()) + "\n")))
}

func (e *EMFCounter) Delete(_ map[string]string) {}

func (e *EMFCounter) DeletePartialMatch(_ map[string]string) {}

func (e *EMF) Reset() {}

type EMFGauge struct {
	*EMF
}

func NewEMFGauge(writer io.Writer, namespace, name string, dimensions [][]string, additionalProperties ...map[string]string) GaugeMetric {
	return &EMFGauge{EMF: NewEMF(writer, namespace, name, dimensions, additionalProperties...)}
}

func (e *EMFGauge) Set(v float64, labels map[string]string) {
	entry := e.withDimensions(e.withProperties(NewEntry(e.namespace), labels), e.dimensions...)
	entry.AddMetric(e.name, v)
	lo.Must(e.writer.Write([]byte(lo.Must(entry.Build()) + "\n")))
}

func (e *EMFGauge) Delete(_ map[string]string) {}

func (e *EMFGauge) DeletePartialMatch(_ map[string]string) {}

func (e *EMFGauge) Reset() {}

type EMFObservation struct {
	*EMF
}

func NewEMFObservation(writer io.Writer, namespace, name string, dimensions [][]string, additionalProperties ...map[string]string) ObservationMetric {
	return &EMFObservation{EMF: NewEMF(writer, namespace, name, dimensions, additionalProperties...)}
}

func (e *EMFObservation) Observe(v float64, labels map[string]string) {
	entry := e.withDimensions(e.withProperties(NewEntry(e.namespace), labels), e.dimensions...)
	entry.AddMetric(e.name, v)
	lo.Must(e.writer.Write([]byte(lo.Must(entry.Build()) + "\n")))
}

func (e *EMFObservation) Delete(_ map[string]string) {

}

func (e *EMFObservation) DeletePartialMatch(_ map[string]string) {

}

func (e *EMFObservation) Reset() {

}

// Copied from https://github.com/aws/aws-sdk-go-v2/blob/v1.32.0/aws/middleware/private/metrics/emf/emf.go#L23
// We needed to make edits to this code since we need to be able to represent different sets of dimensions

const (
	emfIdentifier        = "_aws"
	timestampKey         = "Timestamp"
	cloudWatchMetricsKey = "CloudWatchMetrics"
	namespaceKey         = "Namespace"
	dimensionsKey        = "Dimensions"
	metricsKey           = "Metrics"
)

// Entry represents a log entry in the EMF format.
type Entry struct {
	namespace  string
	metrics    []metric
	dimensions [][]string
	fields     map[string]interface{}
}

type metric struct {
	Name string
}

// NewEntry creates a new Entry with the specified namespace and serializer.
func NewEntry(namespace string) Entry {
	return Entry{
		namespace:  namespace,
		metrics:    []metric{},
		dimensions: [][]string{},
		fields:     map[string]interface{}{},
	}
}

// Build constructs the EMF log entry as a JSON string.
func (e *Entry) Build() (string, error) {

	entry := map[string]interface{}{}

	entry[emfIdentifier] = map[string]interface{}{
		timestampKey: time.Now().UnixNano() / 1e6,
		cloudWatchMetricsKey: []map[string]interface{}{
			{
				namespaceKey:  e.namespace,
				dimensionsKey: e.dimensions,
				metricsKey:    e.metrics,
			},
		},
	}

	for k, v := range e.fields {
		entry[k] = v
	}

	jsonEntry, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	return string(jsonEntry), nil
}

// AddDimensions adds a CW Dimension to the EMF entry.
func (e *Entry) AddDimensions(dimensions ...[]string) {
	// Dimensions are a list of lists. We only support a single list.
	e.dimensions = append(e.dimensions, dimensions...)
}

// AddMetric adds a CW Metric to the EMF entry.
func (e *Entry) AddMetric(key string, value float64) {
	e.metrics = append(e.metrics, metric{key})
	e.fields[key] = value
}

// AddProperty adds a CW Property to the EMF entry.
// Properties are not published as metrics, but they are available in logs and in CW insights.
func (e *Entry) AddProperty(key string, value interface{}) {
	e.fields[key] = value
}
