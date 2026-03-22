package domain

import "context"

type PaymentRepository interface {
	Insert(ctx context.Context, payment *Payment) error
	GetByID(ctx context.Context, id string) (*Payment, error)
	GetByIDForUpdate(ctx context.Context, id string) (*Payment, error)
	List(ctx context.Context, customerID string, cursor string, limit int) ([]*Payment, error)
	UpdateStatus(ctx context.Context, id string, from, to PaymentStatus) error
	InsertEvent(ctx context.Context, event *PaymentEvent) error
	WithTransaction(ctx context.Context, fn func(PaymentRepository) error) error
}
