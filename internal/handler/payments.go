package handler

import (
	"bytes"
	"io"
	"net/http"
	"payments-engine/internal/domain"
	"payments-engine/internal/metrics"
	"payments-engine/internal/service"
	"strconv"
)

func (s *Server) handleCreatePayment(w http.ResponseWriter, r *http.Request) {
	// extract and validate idempotency key
	idempKey := r.Header.Get("Idempotency-Key")
	if idempKey == "" {
		s.respondError(w, r, http.StatusBadRequest, "Idempotency-Key header is required", "missing_idempotency_key")
		return
	}

	// read body once — needed for both hashing and decoding
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.respondError(w, r, http.StatusBadRequest, "failed to read request body", "invalid_request")
		return
	}
	defer r.Body.Close()

	// check idempotency
	result, err := s.idempotencyService.Check(r.Context(), idempKey, body)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	if result.Exists {
		switch {
		case result.RequestHashMismatch:
			s.respondError(w, r, http.StatusUnprocessableEntity, "idempotency key reused with different request body", "idempotency_key_mismatch")
			return
		case result.Status == "pending":
			s.respondError(w, r, http.StatusConflict, "request is still processing", "request_in_flight")
			return
		case result.Status == "completed", result.Status == "failed":
			metrics.IdempotencyHits.Inc()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Idempotency-Replay", "true")
			w.WriteHeader(result.StoredResponseStatus)
			w.Write(result.StoredResponseBody)
			return
		}
	}

	metrics.IdempotencyMisses.Inc()

	// restore body for decoding since we already read it
	r.Body = io.NopCloser(bytes.NewReader(body))

	var req CreatePaymentRequest
	if err := s.decode(w, r, &req); err != nil {
		s.idempotencyService.Fail(r.Context(), idempKey, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
			Code:  "invalid_request",
		})
		return
	}

	// method field validation
	switch req.Method {
	case "card":
		if req.UPI != nil || req.Bank != nil {
			errResp := ErrorResponse{Error: "upi and bank fields not allowed for card payment", Code: "invalid_request"}
			s.idempotencyService.Fail(r.Context(), idempKey, http.StatusBadRequest, errResp)
			s.respondError(w, r, http.StatusBadRequest, errResp.Error, errResp.Code)
			return
		}
		if req.Card == nil {
			errResp := ErrorResponse{Error: "card details required", Code: "invalid_request"}
			s.idempotencyService.Fail(r.Context(), idempKey, http.StatusBadRequest, errResp)
			s.respondError(w, r, http.StatusBadRequest, errResp.Error, errResp.Code)
			return
		}
	case "upi":
		if req.Card != nil || req.Bank != nil {
			errResp := ErrorResponse{Error: "card and bank fields not allowed for upi payment", Code: "invalid_request"}
			s.idempotencyService.Fail(r.Context(), idempKey, http.StatusBadRequest, errResp)
			s.respondError(w, r, http.StatusBadRequest, errResp.Error, errResp.Code)
			return
		}
		if req.UPI == nil {
			errResp := ErrorResponse{Error: "upi details required", Code: "invalid_request"}
			s.idempotencyService.Fail(r.Context(), idempKey, http.StatusBadRequest, errResp)
			s.respondError(w, r, http.StatusBadRequest, errResp.Error, errResp.Code)
			return
		}
	case "bank":
		if req.Card != nil || req.UPI != nil {
			errResp := ErrorResponse{Error: "card and upi fields not allowed for bank payment", Code: "invalid_request"}
			s.idempotencyService.Fail(r.Context(), idempKey, http.StatusBadRequest, errResp)
			s.respondError(w, r, http.StatusBadRequest, errResp.Error, errResp.Code)
			return
		}
		if req.Bank == nil {
			errResp := ErrorResponse{Error: "bank details required", Code: "invalid_request"}
			s.idempotencyService.Fail(r.Context(), idempKey, http.StatusBadRequest, errResp)
			s.respondError(w, r, http.StatusBadRequest, errResp.Error, errResp.Code)
			return
		}
	default:
		errResp := ErrorResponse{Error: "invalid payment method", Code: "invalid_request"}
		s.idempotencyService.Fail(r.Context(), idempKey, http.StatusBadRequest, errResp)
		s.respondError(w, r, http.StatusBadRequest, errResp.Error, errResp.Code)
		return
	}

	input := service.CreatePaymentInput{
		Amount:     req.Amount,
		Currency:   req.Currency,
		CustomerID: req.CustomerID,
		Email:      req.Email,
		IPAddress:  req.IPAddress,
		Method:     req.Method,
		Metadata:   req.Metadata,
	}

	if req.Card != nil {
		input.CardNumber = req.Card.Number
		input.ExpiryMonth = req.Card.ExpiryMonth
		input.ExpiryYear = req.Card.ExpiryYear
		input.CVV = req.Card.CVV
		input.CardholderName = req.Card.CardholderName
	}
	if req.UPI != nil {
		input.UPIID = req.UPI.UPIID
	}
	if req.Bank != nil {
		input.AccountNumber = req.Bank.AccountNumber
		input.IFSCCode = req.Bank.IFSCCode
		input.AccountHolderName = req.Bank.AccountHolderName
	}

	payment, err := s.paymentService.Create(r.Context(), input)
	if err != nil {
		errResp := ErrorResponse{Error: err.Error(), Code: "payment_failed"}
		s.idempotencyService.Fail(r.Context(), idempKey, http.StatusUnprocessableEntity, errResp)
		s.handleError(w, r, err)
		return
	}

	resp := domain.PaymentToResponse(payment)

	if err := s.idempotencyService.Complete(r.Context(), idempKey, payment.ID, http.StatusCreated, resp); err != nil {
		s.logger.ErrorContext(r.Context(), "failed to complete idempotency key", "err", err)
	}

	s.respond(w, r, http.StatusCreated, resp)
}

func (s *Server) handleListPayments(w http.ResponseWriter, r *http.Request) {
	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		s.respondError(w, r, http.StatusBadRequest, "customer_id is required", "invalid_request")
		return
	}

	cursor := r.URL.Query().Get("cursor")

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed <= 0 {
			s.respondError(w, r, http.StatusBadRequest, "invalid limit", "invalid_request")
			return
		}
		limit = parsed
	}

	payments, err := s.paymentService.List(r.Context(), customerID, cursor, limit)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	// build next cursor from last item
	var nextCursor string
	if len(payments) == limit {
		nextCursor = payments[len(payments)-1].ID
	}

	s.respond(w, r, http.StatusOK, ListPaymentsResponse{
		Data:       toListPaymentResponses(payments),
		NextCursor: nextCursor,
	})
}

func toListPaymentResponses(payments []*domain.Payment) []domain.PaymentResponse {
	result := make([]domain.PaymentResponse, len(payments))
	for i, p := range payments {
		result[i] = domain.PaymentToResponse(p)
	}
	return result
}
func (s *Server) handleGetPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.respondError(w, r, http.StatusBadRequest, "missing payment id", "invalid_request")
		return
	}

	payment, err := s.paymentService.GetByID(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	s.respond(w, r, http.StatusOK, domain.PaymentToResponse(payment))
}
func (s *Server) handleConfirmPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.respondError(w, r, http.StatusBadRequest, "missing payment id", "invalid_request")
		return
	}

	payment, err := s.paymentService.Confirm(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	s.respond(w, r, http.StatusOK, domain.PaymentToResponse(payment))
}
func (s *Server) handleCapturePayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.respondError(w, r, http.StatusBadRequest, "missing payment id", "invalid_request")
		return
	}

	payment, err := s.paymentService.Capture(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	s.respond(w, r, http.StatusOK, domain.PaymentToResponse(payment))
}
func (s *Server) handleCancelPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.respondError(w, r, http.StatusBadRequest, "missing payment id", "invalid_request")
		return
	}

	payment, err := s.paymentService.Cancel(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	s.respond(w, r, http.StatusOK, domain.PaymentToResponse(payment))
}
func (s *Server) handleRefundPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.respondError(w, r, http.StatusBadRequest, "missing payment id", "invalid_request")
		return
	}

	payment, err := s.paymentService.Refund(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}

	s.respond(w, r, http.StatusOK, domain.PaymentToResponse(payment))
}
