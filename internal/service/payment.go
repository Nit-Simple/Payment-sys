package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"payments-engine/internal/domain"
	"payments-engine/internal/metrics"
	"payments-engine/pkg/validator"
)

type PaymentService struct {
	repo          domain.PaymentRepository
	encryptionKey []byte
}

func NewPaymentService(repo domain.PaymentRepository, encryptionKey []byte) *PaymentService {
	return &PaymentService{
		repo:          repo,
		encryptionKey: encryptionKey,
	}
}

type CreatePaymentInput struct {
	Amount     int64
	Currency   string
	CustomerID string
	Email      string
	IPAddress  string
	Method     string

	// card
	CardNumber     string
	ExpiryMonth    int
	ExpiryYear     int
	CVV            string
	CardholderName string

	// upi
	UPIID string

	// bank
	AccountNumber     string
	IFSCCode          string
	AccountHolderName string

	Metadata map[string]any
}

func (s *PaymentService) Create(ctx context.Context, input CreatePaymentInput) (*domain.Payment, error) {
	// validate amount
	if err := validator.ValidateAmount(input.Amount); err != nil {
		metrics.ErrorsTotal.WithLabelValues("invalid_amount").Inc()
		return nil, domain.ErrInvalidAmount
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("create payment: generate id: %w", err)
	}

	payment := &domain.Payment{
		ID:         id.String(),
		CustomerID: input.CustomerID,
		Amount:     input.Amount,
		Currency:   input.Currency,
		Status:     domain.StatusInitiated,
		Method:     domain.PaymentMethod(input.Method),
		Email:      input.Email,
		IPAddress:  input.IPAddress,
		Metadata:   input.Metadata,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	// method specific validation and data preparation
	switch payment.Method {
	case domain.MethodCard:
		errs := validator.ValidateCard(input.CardNumber, input.ExpiryMonth, input.ExpiryYear, input.CVV, input.CardholderName)
		if errs.HasErrors() {
			metrics.ErrorsTotal.WithLabelValues("invalid_card").Inc()
			return nil, domain.ErrValidation
		}

		encrypted, err := s.encryptCardData(input.CardNumber, input.CVV, input.CardholderName)
		if err != nil {
			return nil, fmt.Errorf("create payment: encrypt card: %w", err)
		}

		payment.CardLastFour = input.CardNumber[len(input.CardNumber)-4:]
		payment.CardBrand = detectCardBrand(input.CardNumber)
		payment.CardFingerprint = fingerprintCard(input.CardNumber)
		payment.EncryptedCardData = encrypted

	case domain.MethodUPI:
		if err := validator.ValidateUPI(input.UPIID); err != nil {
			metrics.ErrorsTotal.WithLabelValues("invalid_upi").Inc()
			return nil, domain.ErrInvalidUPIID
		}
		payment.UPIID = input.UPIID

	case domain.MethodBank:
		errs := validator.ValidateBankDetails(input.AccountNumber, input.IFSCCode, input.AccountHolderName)
		if errs.HasErrors() {
			metrics.ErrorsTotal.WithLabelValues("invalid_bank").Inc()
			return nil, fmt.Errorf("create payment: %w", errs)
		}
		payment.AccountNumber = input.AccountNumber
		payment.IFSCCode = input.IFSCCode
		payment.AccountHolderName = input.AccountHolderName

	default:
		return nil, fmt.Errorf("create payment: unknown method: %s", input.Method)
	}

	if err := s.repo.Insert(ctx, payment); err != nil {
		metrics.ErrorsTotal.WithLabelValues("db_insert").Inc()
		return nil, fmt.Errorf("create payment: %w", err)
	}

	// record initial event — no from_status for first event
	if err := s.createEvent(ctx, payment.ID, "", domain.StatusInitiated, "payment created"); err != nil {
		metrics.ErrorsTotal.WithLabelValues("db_insert_event").Inc()
		return nil, fmt.Errorf("create payment event: %w", err)
	}

	metrics.PaymentsTotal.WithLabelValues(
		string(payment.Method),
		string(payment.Status),
	).Inc()

	metrics.PaymentAmount.WithLabelValues(
		string(payment.Method),
	).Observe(float64(payment.Amount))
	return payment, nil
}

// encryptCardData encrypts card number and CVV using AES-256-GCM
func (s *PaymentService) encryptCardData(cardNumber, cvv, cardholderName string) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// never store CVV — only encrypt card number and cardholder name
	plaintext := fmt.Sprintf("%s|%s", cardNumber, cardholderName)
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	return ciphertext, nil
}

// fingerprintCard returns SHA-256 of card number for dedup
func fingerprintCard(cardNumber string) string {
	hash := sha256.Sum256([]byte(cardNumber))
	return hex.EncodeToString(hash[:])
}

// detectCardBrand returns card network from card number prefix
func detectCardBrand(cardNumber string) string {
	if len(cardNumber) < 2 {
		return "unknown"
	}

	switch {
	case cardNumber[0] == '4':
		return "visa"
	case cardNumber[:2] >= "51" && cardNumber[:2] <= "55":
		return "mastercard"
	case cardNumber[:2] == "34" || cardNumber[:2] == "37":
		return "amex"
	case cardNumber[:4] == "6011" || cardNumber[:2] == "65":
		return "discover"
	case cardNumber[:4] == "3088" || cardNumber[:4] == "3096":
		return "rupay"
	default:
		return "unknown"
	}
}
func (s *PaymentService) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	payment, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}
	return payment, nil
}
func (s *PaymentService) List(ctx context.Context, customerID string, cursor string, limit int) ([]*domain.Payment, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	payments, err := s.repo.List(ctx, customerID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}

	return payments, nil
}

func (s *PaymentService) createEvent(ctx context.Context, paymentID string, from, to domain.PaymentStatus, reason string) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate event id: %w", err)
	}

	event := &domain.PaymentEvent{
		ID:         id.String(),
		PaymentID:  paymentID,
		FromStatus: from,
		ToStatus:   to,
		Reason:     reason,
		CreatedAt:  time.Now().UTC(),
	}

	return s.repo.InsertEvent(ctx, event)
}

// service/payment.go

func (s *PaymentService) transition(
	ctx context.Context,
	id string,
	to domain.PaymentStatus,
	reason string,
) (*domain.Payment, error) {
	var updated *domain.Payment

	err := s.repo.WithTransaction(ctx, func(tx domain.PaymentRepository) error {
		// SELECT FOR UPDATE — row locked until transaction commits
		payment, err := tx.GetByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}

		if !payment.CanTransitionTo(to) {
			return domain.ErrInvalidStateTransition
		}

		if err := tx.UpdateStatus(ctx, id, payment.Status, to); err != nil {
			return err
		}

		eventID, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate event id: %w", err)
		}

		if err := tx.InsertEvent(ctx, &domain.PaymentEvent{
			ID:         eventID.String(),
			PaymentID:  id,
			FromStatus: payment.Status,
			ToStatus:   to,
			Reason:     reason,
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			metrics.ErrorsTotal.WithLabelValues("db_insert").Inc()
			return err
		}

		payment.Status = to
		updated = payment
		return nil
	})

	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("transition_" + string(to)).Inc()
		return nil, fmt.Errorf("%s payment: %w", reason, err)
	}

	return updated, nil
}

func (s *PaymentService) Confirm(ctx context.Context, id string) (*domain.Payment, error) {
	return s.transition(ctx, id, domain.StatusProcessing, "payment confirmed")
}

func (s *PaymentService) Capture(ctx context.Context, id string) (*domain.Payment, error) {
	return s.transition(ctx, id, domain.StatusSucceeded, "payment captured")
}

func (s *PaymentService) Cancel(ctx context.Context, id string) (*domain.Payment, error) {
	return s.transition(ctx, id, domain.StatusCancelled, "payment cancelled")
}

func (s *PaymentService) Refund(ctx context.Context, id string) (*domain.Payment, error) {
	return s.transition(ctx, id, domain.StatusRefunded, "payment refunded")
}
