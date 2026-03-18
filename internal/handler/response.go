package handler

import "time"

type CreatePaymentResponse struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	Status     string `json:"status"`
	Method     string `json:"method"`

	// card — only last four returned, never full number
	CardLastFour string `json:"card_last_four,omitempty"`
	CardBrand    string `json:"card_brand,omitempty"`

	// upi
	UPIID string `json:"upi_id,omitempty"`

	// bank
	IFSCCode          string `json:"ifsc_code,omitempty"`
	AccountHolderName string `json:"account_holder_name,omitempty"`

	Email     string         `json:"email"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}
type ListPaymentsResponse struct {
	Data       []CreatePaymentResponse `json:"data"`
	NextCursor string                  `json:"next_cursor,omitempty"`
}
