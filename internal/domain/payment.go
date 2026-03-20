package domain

import (
	"time"
)

type PaymentStatus string
type PaymentMethod string

const (
	StatusInitiated  PaymentStatus = "initiated"
	StatusProcessing PaymentStatus = "processing"
	StatusSucceeded  PaymentStatus = "succeeded"
	StatusFailed     PaymentStatus = "failed"
	StatusCancelled  PaymentStatus = "cancelled"
	StatusRefunded   PaymentStatus = "refunded"
)

const (
	MethodCard PaymentMethod = "card"
	MethodUPI  PaymentMethod = "upi"
	MethodBank PaymentMethod = "bank"
)

type Payment struct {
	ID         string
	CustomerID string
	Amount     int64
	Currency   string
	Status     PaymentStatus
	Method     PaymentMethod

	// card
	CardLastFour      string
	CardBrand         string
	CardFingerprint   string
	EncryptedCardData []byte

	// upi
	UPIID string

	// bank
	AccountNumber     string
	IFSCCode          string
	AccountHolderName string

	// common
	Email     string
	IPAddress string
	Metadata  map[string]any

	CreatedAt time.Time
	UpdatedAt time.Time
}

type PaymentEvent struct {
	ID         string
	PaymentID  string
	FromStatus PaymentStatus
	ToStatus   PaymentStatus
	Reason     string
	Metadata   map[string]any
	CreatedAt  time.Time
}

type IdempotencyKey struct {
	Key            string
	PaymentID      string
	RequestHash    string
	Status         string
	ResponseStatus int
	ResponseBody   []byte
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

func (p *Payment) CanTransitionTo(next PaymentStatus) bool {
	switch p.Status {
	case StatusInitiated:
		return next == StatusProcessing || next == StatusCancelled
	case StatusProcessing:
		return next == StatusSucceeded || next == StatusFailed || next == StatusCancelled
	case StatusSucceeded:
		return next == StatusRefunded
	default:
		return false
	}
}
