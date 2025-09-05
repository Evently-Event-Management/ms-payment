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
	OrderID     string            `json:"order_id" binding:"required"`
	PaymentID   string            `json:"payment_id,omitempty"`
	Token       string            `json:"token,omitempty"`
	Card        *StripeCard       `json:"card,omitempty"`
	Amount      float64           `json:"amount,omitempty"` // Made optional
	Currency    string            `json:"currency" default:"usd"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StripePaymentResponse represents a response from a successful Stripe payment
type StripePaymentResponse struct {
	PaymentID     string        `json:"payment_id"`
	OrderID       string        `json:"order_id"`
	Status        PaymentStatus `json:"status"`
	Amount        float64       `json:"amount"`
	Currency      string        `json:"currency"`
	TransactionID string        `json:"transaction_id,omitempty"`
	PaymentMethod string        `json:"payment_method,omitempty"`
	ReceiptURL    string        `json:"receipt_url,omitempty"`
	Created       int64         `json:"created"`
}

// StripeCardValidationRequest represents a request to validate a credit card
type StripeCardValidationRequest struct {
	OrderID string             `json:"order_id" binding:"required"` // Added OrderID to associate with an order
	Card    *StripeCardDetails `json:"card" binding:"required"`
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
	OrderID string `json:"order_id" binding:"required"` // We'll use order_id to fetch payment details from DB
	Reason  string `json:"reason,omitempty"`            // Only reason is needed from client
}

type StripeCard struct {
	Number   string         `json:"number" binding:"required"`
	ExpMonth string         `json:"exp_month" binding:"required"`
	ExpYear  string         `json:"exp_year" binding:"required"`
	CVC      string         `json:"cvc" binding:"required"`
	Name     string         `json:"name,omitempty"`
	Address  *StripeAddress `json:"address,omitempty"`
}
