package observability

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	application string
	requests    *prometheus.CounterVec
	duration    *prometheus.HistogramVec
	registry    *prometheus.Registry
}

func New(application string, pool *pgxpool.Pool) *Metrics {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_server_requests_total",
		Help: "Total HTTP requests handled by the incident service.",
	}, []string{"application", "status", "method", "route"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_server_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"application", "status", "method", "route"})
	registry.MustRegister(requests, duration, collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	if pool != nil {
		registry.MustRegister(newPoolCollector(application, pool))
	}
	return &Metrics{application: application, requests: requests, duration: duration, registry: registry}
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: response, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		route := request.Pattern
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(recorder.status)
		m.requests.WithLabelValues(m.application, status, request.Method, route).Inc()
		m.duration.WithLabelValues(m.application, status, request.Method, route).Observe(time.Since(started).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

type poolCollector struct {
	application string
	pool        *pgxpool.Pool
	waiting     *prometheus.Desc
	emptyTotal  *prometheus.Desc
	acquired    *prometheus.Desc
	idle        *prometheus.Desc
	maximum     *prometheus.Desc
	mu          sync.Mutex
	lastEmpty   int64
}

func newPoolCollector(application string, pool *pgxpool.Pool) *poolCollector {
	labels := []string{"application"}
	return &poolCollector{
		application: application,
		pool:        pool,
		waiting:     prometheus.NewDesc("db_pool_waiting_requests", "Pool acquisitions that began waiting since the previous metrics collection.", labels, nil),
		emptyTotal:  prometheus.NewDesc("db_pool_empty_acquire_total", "Cumulative pool acquisitions that had to wait for a connection.", labels, nil),
		acquired:    prometheus.NewDesc("db_pool_acquired_connections", "Currently acquired PostgreSQL connections.", labels, nil),
		idle:        prometheus.NewDesc("db_pool_idle_connections", "Currently idle PostgreSQL connections.", labels, nil),
		maximum:     prometheus.NewDesc("db_pool_max_connections", "Configured maximum PostgreSQL connections.", labels, nil),
	}
}

func (c *poolCollector) Describe(channel chan<- *prometheus.Desc) {
	channel <- c.waiting
	channel <- c.emptyTotal
	channel <- c.acquired
	channel <- c.idle
	channel <- c.maximum
}

func (c *poolCollector) Collect(channel chan<- prometheus.Metric) {
	stats := c.pool.Stat()
	emptyTotal := stats.EmptyAcquireCount()
	c.mu.Lock()
	waitingSinceLastCollection := emptyTotal - c.lastEmpty
	if waitingSinceLastCollection < 0 {
		waitingSinceLastCollection = 0
	}
	c.lastEmpty = emptyTotal
	c.mu.Unlock()
	channel <- prometheus.MustNewConstMetric(c.waiting, prometheus.GaugeValue, float64(waitingSinceLastCollection), c.application)
	channel <- prometheus.MustNewConstMetric(c.emptyTotal, prometheus.CounterValue, float64(emptyTotal), c.application)
	channel <- prometheus.MustNewConstMetric(c.acquired, prometheus.GaugeValue, float64(stats.AcquiredConns()), c.application)
	channel <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(stats.IdleConns()), c.application)
	channel <- prometheus.MustNewConstMetric(c.maximum, prometheus.GaugeValue, float64(stats.MaxConns()), c.application)
}
