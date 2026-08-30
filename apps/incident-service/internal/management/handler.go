package management

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/store"
)

type Handler struct {
	store       store.HealthStore
	metrics     http.Handler
	application string
	version     string
}

func New(repository store.HealthStore, metrics http.Handler, application, version string) http.Handler {
	handler := &Handler{store: repository, metrics: metrics, application: application, version: version}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /actuator/health", handler.health)
	mux.HandleFunc("GET /actuator/health/readiness", handler.readiness)
	mux.HandleFunc("GET /actuator/health/liveness", handler.liveness)
	mux.HandleFunc("GET /actuator/info", handler.info)
	mux.HandleFunc("GET /actuator/metrics", handler.metricNames)
	mux.Handle("GET /actuator/prometheus", metrics)
	mux.HandleFunc("/actuator/", handler.notFound)
	return mux
}

func (h *Handler) health(response http.ResponseWriter, request *http.Request) {
	h.databaseHealth(response, request)
}

func (h *Handler) readiness(response http.ResponseWriter, request *http.Request) {
	h.databaseHealth(response, request)
}

func (h *Handler) liveness(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "UP"})
}

func (h *Handler) databaseHealth(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := h.store.Ping(ctx); err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]any{
			"status":     "DOWN",
			"components": map[string]any{"db": map[string]any{"status": "DOWN"}},
		})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"status": "UP",
		"components": map[string]any{"db": map[string]any{
			"status": "UP", "details": map[string]string{"database": "PostgreSQL"},
		}},
	})
}

func (h *Handler) info(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{
		"app": map[string]string{"name": h.application, "version": h.version},
	})
}

func (h *Handler) metricNames(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string][]string{"names": {
		"http.server.requests", "db.pool.waiting.requests", "db.pool.acquired.connections",
		"db.pool.idle.connections", "db.pool.max.connections", "process.runtime.go.mem",
	}})
}

func (h *Handler) notFound(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusNotFound, map[string]any{"status": 404, "message": "Management endpoint not found"})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
