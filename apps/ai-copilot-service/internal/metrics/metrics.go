package metrics

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

var defaultBuckets = []float64{
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
	1.5,
	2.5,
	5,
	10,
	30,
}

// InferenceHistogram is a dependency-free Prometheus histogram for the one
// metric consumed by this project's alerts. Counts stores non-cumulative
// bucket values internally and renders Prometheus cumulative values.
type InferenceHistogram struct {
	mu          sync.RWMutex
	application string
	buckets     []float64
	counts      []uint64
	count       uint64
	sum         float64
	max         float64
}

type Snapshot struct {
	Count uint64
	Sum   float64
	Max   float64
}

func NewInferenceHistogram(application string) *InferenceHistogram {
	buckets := append([]float64(nil), defaultBuckets...)
	return &InferenceHistogram{
		application: application,
		buckets:     buckets,
		counts:      make([]uint64, len(buckets)+1),
	}
}

func (histogram *InferenceHistogram) Observe(duration time.Duration) {
	seconds := duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}

	histogram.mu.Lock()
	defer histogram.mu.Unlock()
	histogram.count++
	histogram.sum += seconds
	if seconds > histogram.max {
		histogram.max = seconds
	}
	for index, upperBound := range histogram.buckets {
		if seconds <= upperBound {
			histogram.counts[index]++
			return
		}
	}
	histogram.counts[len(histogram.counts)-1]++
}

func (histogram *InferenceHistogram) Snapshot() Snapshot {
	histogram.mu.RLock()
	defer histogram.mu.RUnlock()
	return Snapshot{Count: histogram.count, Sum: histogram.sum, Max: histogram.max}
}

func (histogram *InferenceHistogram) WritePrometheus(writer io.Writer) error {
	histogram.mu.RLock()
	defer histogram.mu.RUnlock()

	if _, err := io.WriteString(writer, "# HELP ai_triage_inference_seconds Latency distribution of AI Triage and Remediation Inference.\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, "# TYPE ai_triage_inference_seconds histogram\n"); err != nil {
		return err
	}

	application := prometheusLabelValue(histogram.application)
	cumulative := uint64(0)
	for index, upperBound := range histogram.buckets {
		cumulative += histogram.counts[index]
		if _, err := fmt.Fprintf(
			writer,
			"ai_triage_inference_seconds_bucket{application=\"%s\",le=\"%s\"} %d\n",
			application,
			strconv.FormatFloat(upperBound, 'g', -1, 64),
			cumulative,
		); err != nil {
			return err
		}
	}
	cumulative += histogram.counts[len(histogram.counts)-1]
	if _, err := fmt.Fprintf(writer, "ai_triage_inference_seconds_bucket{application=\"%s\",le=\"+Inf\"} %d\n", application, cumulative); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "ai_triage_inference_seconds_sum{application=\"%s\"} %s\n", application, strconv.FormatFloat(histogram.sum, 'g', -1, 64)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "ai_triage_inference_seconds_count{application=\"%s\"} %d\n", application, histogram.count); err != nil {
		return err
	}
	return nil
}

func prometheusLabelValue(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\n", "\\n",
		"\"", "\\\"",
	)
	return replacer.Replace(value)
}
