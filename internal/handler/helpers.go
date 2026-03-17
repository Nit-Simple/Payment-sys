package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"payments-engine/internal/domain"
)

func (s *Server) decode(w http.ResponseWriter, r *http.Request, dst any) error {
	if r.Header.Get("Content-Type") != "application/json" {
		s.respondError(w, r, http.StatusUnsupportedMediaType, "content-type must be application/json", "unsupported_media_type")
		return fmt.Errorf("invalid content type")
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		s.respondError(w, r, http.StatusBadRequest, "invalid request body", "invalid_request")
		return err
	}

	return nil
}

func (s *Server) respond(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode response", "err", err)
	}
}

func (s *Server) respondError(w http.ResponseWriter, r *http.Request, status int, message string, code string) {
	s.respond(w, r, status, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

func (s *Server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrPaymentNotFound):
		s.respondError(w, r, http.StatusNotFound, "payment not found", "payment_not_found")
	case errors.Is(err, domain.ErrInvalidStateTransition):
		s.respondError(w, r, http.StatusUnprocessableEntity, "invalid state transition", "invalid_state_transition")
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		s.respondError(w, r, http.StatusConflict, "duplicate idempotency key", "duplicate_idempotency_key")
	case errors.Is(err, domain.ErrInvalidAmount):
		s.respondError(w, r, http.StatusBadRequest, "invalid amount", "invalid_amount")
	case errors.Is(err, domain.ErrCardExpired):
		s.respondError(w, r, http.StatusBadRequest, "card expired", "card_expired")
	case errors.Is(err, domain.ErrInvalidUPIID):
		s.respondError(w, r, http.StatusBadRequest, "invalid upi id", "invalid_upi_id")
	default:
		s.logger.Error("unhandled error", "err", err)
		s.respondError(w, r, http.StatusInternalServerError, "internal server error", "internal_error")
	}
}
