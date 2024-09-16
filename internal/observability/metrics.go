package observability

import (
	"fmt"
	"log"
	"net/http"
	"registry-service/internal/middleware"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define Prometheus metrics
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status"},
	)

	workerHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "worker_health_status",
			Help: "Health status of workers (1 for healthy, 0 for unhealthy).",
		},
		[]string{"address"},
	)
)

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(workerHealthStatus)
}

// MetricsMiddleware is a middleware to collect metrics for each HTTP request
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start timer
		timer := prometheus.NewTimer(httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, "200"))
		defer timer.ObserveDuration()

		// Capture status code
		ww := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(ww, r)

		// Update Prometheus counters
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprint(ww.status)).Inc()
	})
}

// statusWriter captures the HTTP status code for Prometheus metrics
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// RecordWorkerHealth updates the worker health metric
func RecordWorkerHealth(address string, isHealthy bool) {
	value := 0.0
	if isHealthy {
		value = 1.0
	}
	middleware.GetLogger().Debug("Metrics - Recording health for worker %s: %f\n", address, value)
	workerHealthStatus.WithLabelValues(address).Set(value)
}

var metricsOnce sync.Once

// ServeMetrics starts an HTTP server that exposes the Prometheus metrics endpoint
func ServeMetrics(addr string) {
	metricsOnce.Do(func() {
		http.Handle("/metrics", promhttp.Handler()) // Expose the /metrics endpoint for Prometheus
		middleware.GetLogger().Info("", "Starting registry metrics server on %s\n", addr)

		// Start the HTTP server to expose metrics
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("Error starting metrics server: %v", err)
		}
	})
}
