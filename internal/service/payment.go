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
	"payments-engine/pkg/validator"
)

type PaymentService struct {
	repo          PaymentRepository
	encryptionKey []byte
}

func NewPaymentService(repo PaymentRepository, encryptionKey []byte) *PaymentService {
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
			return nil, domain.ErrInvalidUPIID
		}
		payment.UPIID = input.UPIID

	case domain.MethodBank:
		errs := validator.ValidateBankDetails(input.AccountNumber, input.IFSCCode, input.AccountHolderName)
		if errs.HasErrors() {
			return nil, fmt.Errorf("create payment: %w", errs)
		}
		payment.AccountNumber = input.AccountNumber
		payment.IFSCCode = input.IFSCCode
		payment.AccountHolderName = input.AccountHolderName

	default:
		return nil, fmt.Errorf("create payment: unknown method: %s", input.Method)
	}

	if err := s.repo.Insert(ctx, payment); err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

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
