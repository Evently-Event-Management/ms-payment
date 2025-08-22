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
	PaymentID string        `json:"payment_id"`
	OrderID   string        `json:"order_id"`
	Status    PaymentStatus `json:"status"`
	Price     float64       `json:"price"`
	Date      time.Time     `json:"date"`
}
type PaymentRequest struct {
	PaymentID string        `json:"payment_id"`
	OrderID   string        `json:"order_id"`
	Status    PaymentStatus `json:"status"`
	Price     float64       `json:"price"`
	Date      time.Time     `json:"date"`
}
type PaymentResponse struct {
	ID            string        `json:"id"`
	Status        PaymentStatus `json:"status"`
	Amount        float64       `json:"amount"`
	Currency      string        `json:"currency"`
	TransactionID string        `json:"transaction_id,omitempty"`
	Message       string        `json:"message"`
	CreatedAt     time.Time     `json:"created_at"`
}

type PaymentEvent struct {
	Type      string    `json:"type"`
	PaymentID string    `json:"payment_id"`
	Payment   *Payment  `json:"payment"`
	Timestamp time.Time `json:"timestamp"`
}

type RefundRequest struct {
	Amount string `json:"amount,omitempty"`
	Reason string `json:"reason"`
}

var Req struct {
	Email string `json:"email" binding:"required,email"`
}

type ValidateOTPRequest struct {
	OrderID string `json:"order_id" binding:"required"`
	OTP     string `json:"otp" binding:"required"`
}
