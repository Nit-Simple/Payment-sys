package domain

import "errors"

var (
	ErrPaymentNotFound         = errors.New("payment not found")
	ErrInvalidStateTransition  = errors.New("invalid state transition")
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
	ErrInvalidAmount           = errors.New("invalid amount")
	ErrCardExpired             = errors.New("card expired")
	ErrInvalidUPIID            = errors.New("invalid upi id")
)
