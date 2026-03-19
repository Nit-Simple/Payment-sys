package handler

import (
	"net/http"

	"payments-engine/internal/handler/middleware"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.Handle("GET /metrics", promhttp.Handler())

	mux.HandleFunc("POST /v1/payments", s.handleCreatePayment)
	mux.HandleFunc("GET /v1/payments", s.handleListPayments)
	mux.HandleFunc("GET /v1/payments/{id}", s.handleGetPayment)
	mux.HandleFunc("POST /v1/payments/{id}/confirm", s.handleConfirmPayment)
	mux.HandleFunc("POST /v1/payments/{id}/capture", s.handleCapturePayment)
	mux.HandleFunc("POST /v1/payments/{id}/cancel", s.handleCancelPayment)
	mux.HandleFunc("POST /v1/payments/{id}/refund", s.handleRefundPayment)

	return s.applyMiddleware(mux)
}

func (s *Server) applyMiddleware(h http.Handler) http.Handler {
	h = middleware.RequestLogger(s.logger)(h)
	h = middleware.RequestID(h)
	return h
}
