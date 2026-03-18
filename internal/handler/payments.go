package handler

import (
	"net/http"
	"payments-engine/internal/domain"
	"payments-engine/internal/service"
	"strconv"
)

func (s *Server) handleCreatePayment(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentRequest
	if err := s.decode(w, r, &req); err != nil {
		return
	}

	// method field validation
	switch req.Method {
	case "card":
		if req.UPI != nil || req.Bank != nil {
			s.respondError(w, r, http.StatusBadRequest, "upi and bank fields not allowed for card payment", "invalid_request")
			return
		}
		if req.Card == nil {
			s.respondError(w, r, http.StatusBadRequest, "card details required", "invalid_request")
			return
		}
	case "upi":
		if req.Card != nil || req.Bank != nil {
			s.respondError(w, r, http.StatusBadRequest, "card and bank fields not allowed for upi payment", "invalid_request")
			return
		}
		if req.UPI == nil {
			s.respondError(w, r, http.StatusBadRequest, "upi details required", "invalid_request")
			return
		}
	case "bank":
		if req.Card != nil || req.UPI != nil {
			s.respondError(w, r, http.StatusBadRequest, "card and upi fields not allowed for bank payment", "invalid_request")
			return
		}
		if req.Bank == nil {
			s.respondError(w, r, http.StatusBadRequest, "bank details required", "invalid_request")
			return
		}
	default:
		s.respondError(w, r, http.StatusBadRequest, "invalid payment method", "invalid_request")
		return
	}

	// map request to service input
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
		s.handleError(w, r, err)
		return
	}

	s.respond(w, r, http.StatusCreated, toCreatePaymentResponse(payment))
}

func toCreatePaymentResponse(p *domain.Payment) CreatePaymentResponse {
	return CreatePaymentResponse{
		ID:                p.ID,
		CustomerID:        p.CustomerID,
		Amount:            p.Amount,
		Currency:          p.Currency,
		Status:            string(p.Status),
		Method:            string(p.Method),
		CardLastFour:      p.CardLastFour,
		CardBrand:         p.CardBrand,
		UPIID:             p.UPIID,
		IFSCCode:          p.IFSCCode,
		AccountHolderName: p.AccountHolderName,
		Email:             p.Email,
		Metadata:          p.Metadata,
		CreatedAt:         p.CreatedAt,
	}
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

func toListPaymentResponses(payments []*domain.Payment) []CreatePaymentResponse {
	result := make([]CreatePaymentResponse, len(payments))
	for i, p := range payments {
		result[i] = toCreatePaymentResponse(p)
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

	s.respond(w, r, http.StatusOK, toCreatePaymentResponse(payment))
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

	s.respond(w, r, http.StatusOK, toCreatePaymentResponse(payment))
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

	s.respond(w, r, http.StatusOK, toCreatePaymentResponse(payment))
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

	s.respond(w, r, http.StatusOK, toCreatePaymentResponse(payment))
}
func (s *Server) handleRefundPayment(w http.ResponseWriter, r *http.Request) {}
