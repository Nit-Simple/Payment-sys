package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"payments-engine/internal/domain"
	"payments-engine/internal/metrics"
)

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Insert(ctx context.Context, p *domain.Payment) error {
	query := `
		INSERT INTO payments (
			id, customer_id, amount, currency, status, method,
			card_last_four, card_brand, card_fingerprint, encrypted_card_data,
			upi_id,
			account_number, ifsc_code, account_holder_name,
			email, ip_address, metadata,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11,
			$12, $13, $14,
			$15, $16, $17,
			$18, $19
		)`

	_, err := r.db.Exec(ctx, query,
		p.ID, p.CustomerID, p.Amount, p.Currency, p.Status, p.Method,
		nullableString(p.CardLastFour), nullableString(p.CardBrand),
		nullableString(p.CardFingerprint), nullableBytes(p.EncryptedCardData),
		nullableString(p.UPIID),
		nullableString(p.AccountNumber), nullableString(p.IFSCCode),
		nullableString(p.AccountHolderName),
		p.Email, p.IPAddress, p.Metadata,
		p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}

	return nil
}

func (r *PaymentRepository) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	query := `
    SELECT
        id, customer_id, amount, currency, status, method,
        card_last_four, card_brand, card_fingerprint, encrypted_card_data,
        upi_id,
        account_number, ifsc_code, account_holder_name,
        email, ip_address::text, metadata,
        created_at, updated_at
    FROM payments
    WHERE id = $1`

	p := &domain.Payment{}

	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.CustomerID, &p.Amount, &p.Currency, &p.Status, &p.Method,
		scanNullableString(&p.CardLastFour), scanNullableString(&p.CardBrand),
		scanNullableString(&p.CardFingerprint), scanNullableBytes(&p.EncryptedCardData),
		scanNullableString(&p.UPIID),
		scanNullableString(&p.AccountNumber), scanNullableString(&p.IFSCCode),
		scanNullableString(&p.AccountHolderName),
		&p.Email, &p.IPAddress, &p.Metadata,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("get payment: %w", err)
	}

	return p, nil
}

func (r *PaymentRepository) List(ctx context.Context, customerID string, cursor string, limit int) ([]*domain.Payment, error) {
	query := `
		SELECT
			id, customer_id, amount, currency, status, method,
			card_last_four, card_brand, card_fingerprint, encrypted_card_data,
			upi_id,
			account_number, ifsc_code, account_holder_name,
			email, ip_address::text, metadata,
			created_at, updated_at
		FROM payments
		WHERE customer_id = $1
		AND ($2::uuid IS NULL OR id < $2::uuid)
		ORDER BY id DESC
		LIMIT $3`

	rows, err := r.db.Query(ctx, query, customerID, nullableString(cursor), limit)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	var payments []*domain.Payment

	for rows.Next() {
		p := &domain.Payment{}
		err := rows.Scan(
			&p.ID, &p.CustomerID, &p.Amount, &p.Currency, &p.Status, &p.Method,
			scanNullableString(&p.CardLastFour), scanNullableString(&p.CardBrand),
			scanNullableString(&p.CardFingerprint), scanNullableBytes(&p.EncryptedCardData),
			scanNullableString(&p.UPIID),
			scanNullableString(&p.AccountNumber), scanNullableString(&p.IFSCCode),
			scanNullableString(&p.AccountHolderName),
			&p.Email, &p.IPAddress, &p.Metadata,
			&p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("list payments scan: %w", err)
		}
		payments = append(payments, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list payments rows: %w", err)
	}

	return payments, nil
}

func (r *PaymentRepository) UpdateStatus(ctx context.Context, id string, from, to domain.PaymentStatus) error {
	query := `
        UPDATE payments
        SET status = $1
        WHERE id = $2
        AND status = $3`

	result, err := r.db.Exec(ctx, query, to, id, from)
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("db_update_status").Inc()
		return fmt.Errorf("update payment status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrInvalidStateTransition
	}

	metrics.PaymentStateTransitions.WithLabelValues(
		string(from),
		string(to),
	).Inc()

	return nil
}

func (r *PaymentRepository) InsertEvent(ctx context.Context, e *domain.PaymentEvent) error {
	query := `INSERT INTO payment_events (
            id, payment_id, from_status, to_status, reason, metadata, created_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7
        )`
	_, err := r.db.Exec(ctx, query,
		e.ID,
		e.PaymentID,
		nullableString(string(e.FromStatus)),
		string(e.ToStatus),
		nullableString(e.Reason),
		e.Metadata,
		e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert payment event: %w", err)
	}

	return nil
}

// nullable helpers — domain uses plain Go types, postgres uses nullable types
// these bridge the gap at the repository boundary

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableBytes(b []byte) any {
	if b == nil {
		return nil
	}
	return b
}

func scanNullableString(s *string) any {
	return pgScanString{s}
}

func scanNullableBytes(b *[]byte) any {
	return pgScanBytes{b}
}

type pgScanString struct{ v *string }

func (s pgScanString) Scan(src any) error {
	if src == nil {
		*s.v = ""
		return nil
	}
	switch v := src.(type) {
	case string:
		*s.v = v
	case []byte:
		*s.v = string(v)
	}
	return nil
}

type pgScanBytes struct{ v *[]byte }

func (s pgScanBytes) Scan(src any) error {
	if src == nil {
		*s.v = nil
		return nil
	}
	if v, ok := src.([]byte); ok {
		*s.v = v
	}
	return nil
}
