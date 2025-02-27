package readcloserwithmetrics

import (
	"fmt"
	"io"
	"time"

	"github.com/jonathan-innis/aws-sdk-go-prometheus/v2/awsmetrics"
)

type ReadCloserWithMetrics struct {
	data         *awsmetrics.MetricData
	publisher    awsmetrics.MetricPublisher
	readCloser   io.ReadCloser
	readFinished bool
}

func New(
	context *awsmetrics.MetricContext, closer io.ReadCloser,
) (trc *ReadCloserWithMetrics) {
	return &ReadCloserWithMetrics{
		data:         context.Data(),
		publisher:    context.Publisher(),
		readCloser:   closer,
		readFinished: false,
	}
}

func (r *ReadCloserWithMetrics) Read(p []byte) (n int, err error) {
	readRoundStarted := time.Now().UTC()
	read, err := r.readCloser.Read(p)
	readRoundEnd := time.Now().UTC()
	r.data.Stream.ReadDuration += readRoundEnd.Sub(readRoundStarted)
	r.data.Stream.ReadBytes += int64(read)
	if err == io.EOF {
		r.readFinished = true
		r.finalize()
	}
	return read, err
}

func (r *ReadCloserWithMetrics) Close() error {
	if !r.readFinished {
		r.finalize()
	}
	return r.readCloser.Close()
}

func (r *ReadCloserWithMetrics) finalize() {
	if r.data.Stream.ReadDuration > 0 {
		r.data.Stream.Throughput = float64(r.data.Stream.ReadBytes) / r.data.Stream.ReadDuration.Seconds()
	}
	err := r.publisher.PostStreamMetrics(r.data)
	if err != nil {
		fmt.Println("Failed to post stream metrics")
	}
}
