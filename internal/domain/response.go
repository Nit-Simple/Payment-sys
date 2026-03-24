package domain

import "time"

// PaymentResponse is the canonical API representation of a payment.
// Used by both the handler and the recovery sweep to ensure
// reconstructed responses are identical to what the handler returns.
type PaymentResponse struct {
	ID                string         `json:"id"`
	CustomerID        string         `json:"customer_id"`
	Amount            int64          `json:"amount"`
	Currency          string         `json:"currency"`
	Status            string         `json:"status"`
	Method            string         `json:"method"`
	CardLastFour      string         `json:"card_last_four,omitempty"`
	CardBrand         string         `json:"card_brand,omitempty"`
	UPIID             string         `json:"upi_id,omitempty"`
	IFSCCode          string         `json:"ifsc_code,omitempty"`
	AccountHolderName string         `json:"account_holder_name,omitempty"`
	Email             string         `json:"email"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

func PaymentToResponse(p *Payment) PaymentResponse {
	return PaymentResponse{
		ID:                p.ID,
		CustomerID:        p.CustomerID,
		Amount:            p.Amount,
		Currency:          p.Currency,
		Status:            string(p.Status),
		Method:            string(p.Method),
		CardLastFour:      p.CardLastFour,
		CardBrand:         p.CardBrand,
		UPIID:             p.UPIID,
		IFSCCode:          p.IFSCCode,
		AccountHolderName: p.AccountHolderName,
		Email:             p.Email,
		Metadata:          p.Metadata,
		CreatedAt:         p.CreatedAt,
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}
