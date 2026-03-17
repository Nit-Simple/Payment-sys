package handler

type CreatePaymentRequest struct {
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	IPAddress  string `json:"ip_address"`
	Method     string `json:"method"`

	// card
	Card *CardDetails `json:"card,omitempty"`

	// upi
	UPI *UPIDetails `json:"upi,omitempty"`

	// bank
	Bank *BankDetails `json:"bank,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`
}

type CardDetails struct {
	Number         string `json:"card_number"`
	ExpiryMonth    int    `json:"expiry_month"`
	ExpiryYear     int    `json:"expiry_year"`
	CVV            string `json:"cvv"`
	CardholderName string `json:"cardholder_name"`
}

type UPIDetails struct {
	UPIID string `json:"upi_id"`
}

type BankDetails struct {
	AccountNumber     string `json:"account_number"`
	IFSCCode          string `json:"ifsc_code"`
	AccountHolderName string `json:"account_holder_name"`
}
