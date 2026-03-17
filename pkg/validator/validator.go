package validator

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var (
	upiRegex  = regexp.MustCompile(`^[a-zA-Z0-9._-]+@[a-zA-Z]+$`)
	ifscRegex = regexp.MustCompile(`^[A-Z]{4}0[A-Z0-9]{6}$`)
)

// ValidationError holds all validation failures together
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Fields)
}

func (e *ValidationError) Add(field, message string) {
	if e.Fields == nil {
		e.Fields = make(map[string]string)
	}
	e.Fields[field] = message
}

func (e *ValidationError) HasErrors() bool {
	return len(e.Fields) > 0
}

// ValidateAmount checks amount is positive and under max limit
func ValidateAmount(amount int64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	// 10 lakh rupees in paise as max single transaction limit
	if amount > 100_000_00 {
		return fmt.Errorf("amount exceeds maximum limit")
	}
	return nil
}

// ValidateCard validates all card fields
func ValidateCard(number string, expiryMonth, expiryYear int, cvv, cardholderName string) *ValidationError {
	errs := &ValidationError{}

	if !luhn(number) {
		errs.Add("card_number", "invalid card number")
	}

	if !cardExpiryValid(expiryMonth, expiryYear) {
		errs.Add("expiry", "card is expired")
	}

	if !cvvValid(cvv, number) {
		errs.Add("cvv", "invalid cvv")
	}

	if cardholderName == "" {
		errs.Add("cardholder_name", "cardholder name is required")
	}

	return errs
}

// ValidateUPI validates a UPI ID
func ValidateUPI(upiID string) error {
	if !upiRegex.MatchString(upiID) {
		return fmt.Errorf("invalid upi id")
	}
	return nil
}

// ValidateIFSC validates an IFSC code
func ValidateIFSC(ifsc string) error {
	if !ifscRegex.MatchString(ifsc) {
		return fmt.Errorf("invalid ifsc code")
	}
	return nil
}

// luhn validates a card number using the Luhn algorithm
func luhn(number string) bool {
	sum := 0
	alternate := false

	for i := len(number) - 1; i >= 0; i-- {
		n, err := strconv.Atoi(string(number[i]))
		if err != nil {
			return false
		}

		if alternate {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}

		sum += n
		alternate = !alternate
	}

	return sum%10 == 0
}

// cardExpiryValid checks the card is not expired
func cardExpiryValid(month, year int) bool {
	if month < 1 || month > 12 {
		return false
	}

	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())

	if year < currentYear {
		return false
	}
	if year == currentYear && month < currentMonth {
		return false
	}

	return true
}

// cvvValid checks CVV length based on card type
// Amex requires 4 digits, all others require 3
func cvvValid(cvv, cardNumber string) bool {
	if len(cardNumber) == 0 {
		return false
	}

	// Amex starts with 34 or 37
	if len(cardNumber) >= 2 {
		prefix := cardNumber[:2]
		if prefix == "34" || prefix == "37" {
			return len(cvv) == 4
		}
	}

	return len(cvv) == 3
}
func ValidateBankDetails(accountNumber, ifsc, accountHolderName string) *ValidationError {
	errs := &ValidationError{}

	if accountNumber == "" {
		errs.Add("account_number", "account number is required")
	}

	if err := ValidateIFSC(ifsc); err != nil {
		errs.Add("ifsc_code", err.Error())
	}

	if accountHolderName == "" {
		errs.Add("account_holder_name", "account holder name is required")
	}

	return errs
}
