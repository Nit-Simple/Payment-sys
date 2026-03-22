package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"payments-engine/internal/domain"
	"payments-engine/internal/metrics"
)

type IdempotencyRepository interface {
	InsertKey(ctx context.Context, key *domain.IdempotencyKey) (*domain.IdempotencyKey, error) // no bool
	GetKey(ctx context.Context, key string) (*domain.IdempotencyKey, error)
	UpdateKey(ctx context.Context, key string, paymentID string, status string, responseStatus int, responseBody []byte) error
}

type IdempotencyService struct {
	repo IdempotencyRepository
}

func NewIdempotencyService(repo IdempotencyRepository) *IdempotencyService {
	return &IdempotencyService{repo: repo}
}

// IdempotencyResult is what the service returns after checking the key
type IdempotencyResult struct {
	// Exists means this key was seen before
	Exists bool
	// Claimed means we just claimed this key — proceed with processing
	Claimed bool
	// Status of existing key if it exists
	Status string
	// StoredResponseStatus — HTTP status to return
	StoredResponseStatus int
	// StoredResponseBody — exact response body to return
	StoredResponseBody []byte
	// RequestHashMismatch — same key, different body
	RequestHashMismatch bool
}

func (s *IdempotencyService) Check(ctx context.Context, key string, requestBody []byte) (*IdempotencyResult, error) {
	requestHash := hashBody(requestBody)

	existing, err := s.repo.InsertKey(ctx, &domain.IdempotencyKey{
		Key:         key,
		RequestHash: requestHash,
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour),
	})

	// fresh insert — key is new, proceed with processing
	if err == nil {
		_ = existing // not needed, key is claimed
		return &IdempotencyResult{Claimed: true}, nil
	}

	// conflict — key already exists
	if errors.Is(err, domain.ErrIdempotencyKeyExists) {
		existing, err := s.repo.GetKey(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("idempotency check: get existing key: %w", err)
		}

		if existing.RequestHash != requestHash {
			return &IdempotencyResult{
				Exists:              true,
				RequestHashMismatch: true,
			}, nil
		}

		return &IdempotencyResult{
			Exists:               true,
			Status:               existing.Status,
			StoredResponseStatus: existing.ResponseStatus,
			StoredResponseBody:   existing.ResponseBody,
		}, nil
	}

	// real error
	return nil, fmt.Errorf("idempotency check: %w", err)
}
func (s *IdempotencyService) Complete(ctx context.Context, key string, paymentID string, responseStatus int, responseBody any) error {
	body, err := json.Marshal(responseBody)
	if err != nil {
		return fmt.Errorf("idempotency complete: marshal response: %w", err)
	}

	if err := s.repo.UpdateKey(ctx, key, paymentID, "completed", responseStatus, body); err != nil {
		metrics.ErrorsTotal.WithLabelValues("idempotency_complete").Inc()
		return fmt.Errorf("idempotency complete: %w", err)
	}

	return nil
}

func (s *IdempotencyService) Fail(ctx context.Context, key string, responseStatus int, responseBody any) error {
	body, err := json.Marshal(responseBody)
	if err != nil {
		return fmt.Errorf("idempotency fail: marshal response: %w", err)
	}

	if err := s.repo.UpdateKey(ctx, key, "", "failed", responseStatus, body); err != nil {
		metrics.ErrorsTotal.WithLabelValues("idempotency_fail").Inc()
		return fmt.Errorf("idempotency fail: %w", err)
	}

	return nil
}

func hashBody(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}
