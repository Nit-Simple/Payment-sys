package service

import (
	"context"

	"payments-engine/internal/domain"
)

// PaymentRepository is the interface the service depends on
// defined here in service package, not repository package
type PaymentRepository interface {
	Insert(ctx context.Context, payment *domain.Payment) error
	GetByID(ctx context.Context, id string) (*domain.Payment, error)
	List(ctx context.Context, customerID string, cursor string, limit int) ([]*domain.Payment, error)
	UpdateStatus(ctx context.Context, id string, from, to domain.PaymentStatus) error
}
