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
	ID             string        `json:"id"`
	MerchantID     string        `json:"merchant_id"`
	Amount         float64       `json:"amount"`
	Currency       string        `json:"currency"`
	Status         PaymentStatus `json:"status"`
	PaymentMethod  string        `json:"payment_method"`
	CardNumber     string        `json:"card_number,omitempty"`
	CardHolderName string        `json:"card_holder_name,omitempty"`
	ExpiryMonth    int           `json:"expiry_month,omitempty"`
	ExpiryYear     int           `json:"expiry_year,omitempty"`
	CVV            string        `json:"cvv,omitempty"`
	Description    string        `json:"description,omitempty"`
	CallbackURL    string        `json:"callback_url,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	ProcessedAt    *time.Time    `json:"processed_at,omitempty"`
	ErrorMessage   string        `json:"error_message,omitempty"`
	TransactionID  string        `json:"transaction_id,omitempty"`
}

type PaymentRequest struct {
	MerchantID     string  `json:"merchant_id" binding:"required"`
	Amount         float64 `json:"amount" binding:"required,gt=0"`
	Currency       string  `json:"currency" binding:"required,len=3"`
	PaymentMethod  string  `json:"payment_method" binding:"required"`
	CardNumber     string  `json:"card_number" binding:"required"`
	CardHolderName string  `json:"card_holder_name" binding:"required"`
	ExpiryMonth    int     `json:"expiry_month" binding:"required,min=1,max=12"`
	ExpiryYear     int     `json:"expiry_year" binding:"required,min=2024"`
	CVV            string  `json:"cvv" binding:"required,len=3"`
	Description    string  `json:"description"`
	CallbackURL    string  `json:"callback_url"`
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
