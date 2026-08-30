package metrics

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestInferenceHistogramPrometheusOutput(t *testing.T) {
	histogram := NewInferenceHistogram(`ai-"copilot`)
	histogram.Observe(100 * time.Millisecond)
	histogram.Observe(2 * time.Second)

	var output bytes.Buffer
	if err := histogram.WritePrometheus(&output); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	text := output.String()
	for _, expected := range []string{
		`# TYPE ai_triage_inference_seconds histogram`,
		`ai_triage_inference_seconds_bucket{application="ai-\"copilot",le="0.1"} 1`,
		`ai_triage_inference_seconds_bucket{application="ai-\"copilot",le="2.5"} 2`,
		`ai_triage_inference_seconds_bucket{application="ai-\"copilot",le="+Inf"} 2`,
		`ai_triage_inference_seconds_count{application="ai-\"copilot"} 2`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("metrics missing %q:\n%s", expected, text)
		}
	}

	snapshot := histogram.Snapshot()
	if snapshot.Count != 2 || snapshot.Sum < 2.099 || snapshot.Sum > 2.101 || snapshot.Max != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestInferenceHistogramConcurrentObserve(t *testing.T) {
	histogram := NewInferenceHistogram("ai-copilot-service")
	const goroutines = 20
	const observations = 100
	var group sync.WaitGroup
	for range goroutines {
		group.Add(1)
		go func() {
			defer group.Done()
			for range observations {
				histogram.Observe(time.Millisecond)
			}
		}()
	}
	group.Wait()
	if actual := histogram.Snapshot().Count; actual != goroutines*observations {
		t.Fatalf("count = %d, want %d", actual, goroutines*observations)
	}
}
