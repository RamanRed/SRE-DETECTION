package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/metrics"
)

const prometheusContentType = "text/plain; version=0.0.4; charset=utf-8"

type Handler struct {
	application string
	latency     *metrics.InferenceHistogram
	ready       atomic.Bool
	mux         *http.ServeMux
}

func NewHandler(application string, latency *metrics.InferenceHistogram) *Handler {
	handler := &Handler{application: application, latency: latency, mux: http.NewServeMux()}
	handler.mux.HandleFunc("/actuator", handler.actuator)
	handler.mux.HandleFunc("/actuator/health", handler.health)
	handler.mux.HandleFunc("/actuator/health/liveness", handler.liveness)
	handler.mux.HandleFunc("/actuator/health/readiness", handler.readiness)
	handler.mux.HandleFunc("/actuator/info", handler.info)
	handler.mux.HandleFunc("/actuator/prometheus", handler.prometheus)
	handler.mux.HandleFunc("/actuator/metrics", handler.metricNames)
	handler.mux.HandleFunc("/actuator/metrics/ai.triage.inference", handler.inferenceMetric)
	// Idiomatic aliases for non-Spring deployments and generic orchestrators.
	handler.mux.HandleFunc("/healthz", handler.liveness)
	handler.mux.HandleFunc("/readyz", handler.readiness)
	handler.mux.HandleFunc("/metrics", handler.prometheus)
	return handler
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.mux.ServeHTTP(writer, request)
}

func (handler *Handler) SetReady(ready bool) {
	handler.ready.Store(ready)
}

func (handler *Handler) IsReady() bool {
	return handler.ready.Load()
}

func (handler *Handler) actuator(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"_links": map[string]any{
			"self":       map[string]string{"href": "/actuator"},
			"health":     map[string]string{"href": "/actuator/health"},
			"info":       map[string]string{"href": "/actuator/info"},
			"metrics":    map[string]string{"href": "/actuator/metrics"},
			"prometheus": map[string]string{"href": "/actuator/prometheus"},
			"healthz":    map[string]string{"href": "/healthz"},
			"readyz":     map[string]string{"href": "/readyz"},
		},
	})
}

func (handler *Handler) health(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	readinessStatus := "OUT_OF_SERVICE"
	if handler.ready.Load() {
		readinessStatus = "UP"
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"status": "UP",
		"components": map[string]any{
			"livenessState":  map[string]string{"status": "UP"},
			"readinessState": map[string]string{"status": readinessStatus},
		},
	})
}

func (handler *Handler) liveness(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "UP"})
}

func (handler *Handler) readiness(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	if !handler.ready.Load() {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "OUT_OF_SERVICE"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "UP"})
}

func (handler *Handler) info(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"app": map[string]string{
			"name":    handler.application,
			"version": "1.0.0",
		},
	})
}

func (handler *Handler) prometheus(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	writer.Header().Set("Content-Type", prometheusContentType)
	if err := handler.latency.WritePrometheus(writer); err != nil {
		// At this point headers may already be committed; closing the connection is
		// the best the HTTP server can do, and the next scrape will retry.
		return
	}
}

func (handler *Handler) metricNames(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string][]string{"names": {"ai.triage.inference"}})
}

func (handler *Handler) inferenceMetric(writer http.ResponseWriter, request *http.Request) {
	if !allowGet(writer, request) {
		return
	}
	snapshot := handler.latency.Snapshot()
	writeJSON(writer, http.StatusOK, map[string]any{
		"name": "ai.triage.inference",
		"measurements": []map[string]any{
			{"statistic": "COUNT", "value": snapshot.Count},
			{"statistic": "TOTAL_TIME", "value": snapshot.Sum},
			{"statistic": "MAX", "value": snapshot.Max},
		},
		"availableTags": []map[string]any{
			{"tag": "application", "values": []string{handler.application}},
		},
	})
}

func allowGet(writer http.ResponseWriter, request *http.Request) bool {
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		return true
	}
	writer.Header().Set("Allow", strings.Join([]string{http.MethodGet, http.MethodHead}, ", "))
	http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	return false
}

func writeJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(body); err != nil {
		_, _ = fmt.Fprint(writer, "{}")
	}
}
