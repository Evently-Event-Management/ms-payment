package models

import (
	"time"
)

type PaymentStatus string

const (
	StatusPending   PaymentStatus = "pending"
	StatusSuccess   PaymentStatus = "success"
	StatusFailed    PaymentStatus = "failed"
	StatusRefunded  PaymentStatus = "refunded"
	StatusCancelled PaymentStatus = "cancelled"
)

type Payment struct {
	PaymentID     string        `json:"payment_id" bun:"payment_id,pk"`
	OrderID       string        `json:"order_id" bun:"order_id"`
	Status        PaymentStatus `json:"status" bun:"status"`
	Price         float64       `json:"price" bun:"price"` // Added price field
	CreatedDate   time.Time     `json:"created_date" bun:"created_date"`
	URL           string        `json:"url" bun:"url"`
	TransactionID string        `json:"transaction_id,omitempty" bun:"transaction_id,nullzero"`
	UpdatedDate   time.Time     `json:"updated_date,omitempty" bun:"updated_date,nullzero"`
}

type PaymentRequest struct {
	PaymentID string        `json:"payment_id,omitempty"` // Added PaymentID field
	OrderID   string        `json:"order_id"`
	Status    PaymentStatus `json:"status"`
	Price     float64       `json:"price,omitempty"` // Made price optional
	URL       string        `json:"url"`
	Source    string        `json:"source,omitempty"` // Source of the payment (e.g., "stripe", "manual")
}

type PaymentResponse struct {
	PaymentID   string        `json:"payment_id"`
	OrderID     string        `json:"order_id"`
	Status      PaymentStatus `json:"status"`
	Price       float64       `json:"price"` // Added price field
	CreatedDate time.Time     `json:"created_date"`
	URL         string        `json:"url"`
}

type PaymentEvent struct {
	Type      string    `json:"type"`
	PaymentID string    `json:"payment_id"`
	OrderID   string    `json:"order_id"`
	Payment   *Payment  `json:"payment"`
	Timestamp time.Time `json:"timestamp"`
}

type PaymentStreamRequest struct {
	PaymentID string `json:"payment_id" binding:"required"`
	Status    string `json:"status,omitempty"`
	EventType string `json:"event_type,omitempty"`
}

type RefundRequest struct {
	OrderID string `json:"order_id,omitempty"` // Added OrderID field to replace PaymentID
	Amount  string `json:"amount,omitempty"`
	Reason  string `json:"reason"`
}

var Req struct {
	Email string `json:"email" binding:"required,email"`
}

type ValidateOTPRequest struct {
	OrderID string `json:"order_id" binding:"required"`
	OTP     string `json:"otp" binding:"required"`
}
