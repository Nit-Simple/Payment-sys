package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path and status",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "path"},
	)

	HTTPRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		},
	)

	// Payment metrics
	PaymentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_total",
			Help: "Total number of payments by method and status",
		},
		[]string{"method", "status"},
	)

	PaymentAmount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "payment_amount_paise",
			Help:    "Distribution of payment amounts in paise",
			Buckets: []float64{10000, 50000, 100000, 500000, 1000000, 5000000, 10000000},
		},
		[]string{"method"},
	)

	PaymentStateTransitions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payment_state_transitions_total",
			Help: "Total number of payment state transitions",
		},
		[]string{"from", "to"},
	)

	PaymentProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "payment_processing_duration_seconds",
			Help:    "Time taken to process a payment from initiated to terminal state",
			Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"method", "terminal_status"},
	)

	// Database metrics
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)

	DBConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_in_use",
			Help: "Number of database connections currently in use",
		},
	)

	DBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections in the pool",
		},
	)

	// Error metrics
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total number of errors by type",
		},
		[]string{"type"},
	)

	// Idempotency metrics
	IdempotencyHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "idempotency_hits_total",
			Help: "Total number of duplicate requests caught by idempotency layer",
		},
	)

	IdempotencyMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "idempotency_misses_total",
			Help: "Total number of new requests passing through idempotency layer",
		},
	)
)
