package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/metrics"
)

func TestHealthEndpointsAndReadinessLifecycle(t *testing.T) {
	histogram := metrics.NewInferenceHistogram("ai-copilot-service")
	handler := NewHandler("ai-copilot-service", histogram)

	assertStatusAndBody(t, handler, "/actuator/health/liveness", http.StatusOK, `"status":"UP"`)
	assertStatusAndBody(t, handler, "/healthz", http.StatusOK, `"status":"UP"`)
	assertStatusAndBody(t, handler, "/actuator/health/readiness", http.StatusServiceUnavailable, `"status":"OUT_OF_SERVICE"`)
	assertStatusAndBody(t, handler, "/readyz", http.StatusServiceUnavailable, `"status":"OUT_OF_SERVICE"`)
	assertStatusAndBody(t, handler, "/actuator/health", http.StatusOK, `"readinessState":{"status":"OUT_OF_SERVICE"}`)

	handler.SetReady(true)
	if !handler.IsReady() {
		t.Fatal("handler did not become ready")
	}
	assertStatusAndBody(t, handler, "/actuator/health/readiness", http.StatusOK, `"status":"UP"`)
	assertStatusAndBody(t, handler, "/readyz", http.StatusOK, `"status":"UP"`)
	assertStatusAndBody(t, handler, "/actuator/health", http.StatusOK, `"readinessState":{"status":"UP"}`)
}

func TestPrometheusAndLegacyMetricsEndpoints(t *testing.T) {
	histogram := metrics.NewInferenceHistogram("ai-copilot-service")
	histogram.Observe(250 * time.Millisecond)
	handler := NewHandler("ai-copilot-service", histogram)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/actuator/prometheus", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("prometheus status = %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != prometheusContentType {
		t.Fatalf("content type = %q", contentType)
	}
	if !strings.Contains(recorder.Body.String(), `ai_triage_inference_seconds_count{application="ai-copilot-service"} 1`) {
		t.Fatalf("unexpected Prometheus output:\n%s", recorder.Body.String())
	}
	assertStatusAndBody(t, handler, "/metrics", http.StatusOK, `ai_triage_inference_seconds_count{application="ai-copilot-service"} 1`)

	assertStatusAndBody(t, handler, "/actuator/metrics", http.StatusOK, `"ai.triage.inference"`)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/actuator/metrics/ai.triage.inference", nil))
	var body struct {
		Name         string `json:"name"`
		Measurements []struct {
			Statistic string  `json:"statistic"`
			Value     float64 `json:"value"`
		} `json:"measurements"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode metric response: %v", err)
	}
	if body.Name != "ai.triage.inference" || len(body.Measurements) != 3 || body.Measurements[0].Value != 1 {
		t.Fatalf("unexpected metric response: %+v", body)
	}
}

func TestActuatorInfoAndMethodHandling(t *testing.T) {
	handler := NewHandler("ai-copilot-service", metrics.NewInferenceHistogram("ai-copilot-service"))
	assertStatusAndBody(t, handler, "/actuator", http.StatusOK, `"prometheus"`)
	assertStatusAndBody(t, handler, "/actuator/info", http.StatusOK, `"name":"ai-copilot-service"`)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/actuator/health", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") == "" {
		t.Fatalf("method response = %d, Allow=%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}

func assertStatusAndBody(t *testing.T, handler http.Handler, path string, status int, fragment string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != status {
		t.Fatalf("GET %s status = %d, want %d: %s", path, recorder.Code, status, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), fragment) {
		t.Fatalf("GET %s body missing %q: %s", path, fragment, recorder.Body.String())
	}
}
