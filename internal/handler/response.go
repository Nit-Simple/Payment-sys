package handler

import (
	"payments-engine/internal/domain"
)

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}
type ListPaymentsResponse struct {
	Data       []domain.PaymentResponse `json:"data"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}
