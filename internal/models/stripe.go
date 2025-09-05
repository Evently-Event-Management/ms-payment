package models

// StripeCardDetails represents credit card information
type StripeCardDetails struct {
	Number   string         `json:"number" binding:"required"`
	ExpMonth string         `json:"exp_month" binding:"required"`
	ExpYear  string         `json:"exp_year" binding:"required"`
	CVC      string         `json:"cvc" binding:"required"`
	Name     string         `json:"name"`
	Address  *StripeAddress `json:"address,omitempty"`
}

// StripeAddress represents billing address information
type StripeAddress struct {
	Line1      string `json:"line1,omitempty"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country,omitempty"`
}

// StripePaymentRequest represents a request to process a payment through Stripe
type StripePaymentRequest struct {
	PaymentID   string             `json:"payment_id" binding:"required"`
	OrderID     string             `json:"order_id" binding:"required"`
	Amount      float64            `json:"amount" binding:"required,gt=0"`
	Currency    string             `json:"currency" binding:"required"`
	Description string             `json:"description,omitempty"`
	Token       string             `json:"token,omitempty"` // Stripe token or PaymentMethod ID
	Card        *StripeCardDetails `json:"card,omitempty"`  // Optional for legacy/test
	Metadata    map[string]string  `json:"metadata,omitempty"`
}

// StripePaymentResponse represents a response from a successful Stripe payment
type StripePaymentResponse struct {
	PaymentID     string        `json:"payment_id"`
	OrderID       string        `json:"order_id"`
	Status        PaymentStatus `json:"status"`
	Amount        float64       `json:"amount"`
	Currency      string        `json:"currency"`
	TransactionID string        `json:"transaction_id"`
	PaymentMethod string        `json:"payment_method"`
	ReceiptURL    string        `json:"receipt_url,omitempty"`
	Created       int64         `json:"created"`
}

// StripeCardValidationRequest represents a request to validate a credit card
type StripeCardValidationRequest struct {
	Card *StripeCardDetails `json:"card" binding:"required"`
}

// StripeCardValidationResponse represents the response from a card validation request
type StripeCardValidationResponse struct {
	Valid    bool   `json:"valid"`
	Message  string `json:"message,omitempty"`
	CardType string `json:"card_type,omitempty"`
	Last4    string `json:"last4,omitempty"`
}

// StripeRefundRequest represents a request to refund a payment
type StripeRefundRequest struct {
	PaymentID string   `json:"payment_id" binding:"required"`
	Amount    *float64 `json:"amount,omitempty"`
	Reason    string   `json:"reason,omitempty"`
}

type StripeCard struct {
	Number   string
	ExpMonth string
	ExpYear  string
	CVC      string
	Name     string
	Address  *StripeAddress
}
