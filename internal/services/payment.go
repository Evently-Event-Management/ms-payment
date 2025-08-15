package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	otp2 "payment-gateway/internal/otp"
	"time"

	"payment-gateway/internal/kafka"
	"payment-gateway/internal/logger"
	"payment-gateway/internal/models"
	"payment-gateway/internal/storage"
	"payment-gateway/internal/utils"
)

var (
	ErrPaymentNotFound      = errors.New("payment not found")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidCard          = errors.New("invalid card details")
	ErrCardExpired          = errors.New("card expired")
	ErrPaymentDeclined      = errors.New("payment declined")
	ErrInvalidRefundAmount  = errors.New("invalid refund amount")
	ErrPaymentNotRefundable = errors.New("payment not refundable")
)

type PaymentService struct {
	store    storage.Store
	producer *kafka.Producer
	log      *logger.Logger // Added logger to service
}

func NewPaymentService(store storage.Store, producer *kafka.Producer, log *logger.Logger) *PaymentService {
	return &PaymentService{
		store:    store,
		producer: producer,
		log:      log,
	}
}

func (s *PaymentService) OtpSender(email string) {

	// Simulate sending OTP
	otp, _ := otp2.GenerateOTP()
	otp2.SendEmailOTP(email, otp)
	// Here you would integrate with an email service to send the OTP
	// For now, we just log it
	s.log.Info("OTP", fmt.Sprintf("Sent OTP to %s: %s", email, otp))

}
func (s *PaymentService) ProcessPayment(ctx context.Context, req *models.PaymentRequest) (*models.Payment, error) {
	s.log.LogPayment("INIT", "new", fmt.Sprintf("Processing payment for merchant %s, amount: %.2f %s",
		req.MerchantID, req.Amount, req.Currency))

	// Create payment record
	payment := &models.Payment{
		ID:             utils.GenerateID(),
		MerchantID:     req.MerchantID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         models.StatusPending,
		PaymentMethod:  req.PaymentMethod,
		CardNumber:     s.maskCardNumber(req.CardNumber),
		CardHolderName: req.CardHolderName,
		ExpiryMonth:    req.ExpiryMonth,
		ExpiryYear:     req.ExpiryYear,
		Description:    req.Description,
		CallbackURL:    req.CallbackURL,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	s.log.LogPayment("CREATE", payment.ID, fmt.Sprintf("Payment record created with status: %s", payment.Status))

	// Save payment to storage
	if err := s.store.SavePayment(payment); err != nil {
		s.log.Error("PAYMENT", fmt.Sprintf("Failed to save payment %s: %v", payment.ID, err))
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	s.log.LogDatabase("SAVE", "payments", fmt.Sprintf("Payment %s saved successfully", payment.ID))

	// Simulate payment processing
	go s.processPaymentAsync(ctx, payment, req)

	s.log.LogPayment("ASYNC", payment.ID, "Payment processing started asynchronously")
	return payment, nil
}

func (s *PaymentService) processPaymentAsync(ctx context.Context, payment *models.Payment, req *models.PaymentRequest) {
	s.log.LogProcess("ASYNC_PAYMENT", fmt.Sprintf("Starting async processing for payment %s", payment.ID))

	// Simulate processing delay
	processingTime := time.Duration(rand.Intn(3)+1) * time.Second
	s.log.LogPayment("PROCESSING", payment.ID, fmt.Sprintf("Simulating processing delay: %v", processingTime))
	time.Sleep(processingTime)

	// Validate card details
	s.log.LogPayment("VALIDATE", payment.ID, "Validating card details...")
	if err := s.validateCard(req); err != nil {
		s.log.LogPayment("VALIDATION_FAILED", payment.ID, fmt.Sprintf("Card validation failed: %v", err))
		s.updatePaymentStatus(payment, models.StatusFailed, err.Error())
		s.publishPaymentEvent("payment.failed", payment)
		return
	}

	s.log.LogPayment("VALIDATION_SUCCESS", payment.ID, "Card validation successful")

	// Simulate payment gateway response
	if s.shouldPaymentSucceed(req) {
		s.log.LogPayment("SUCCESS", payment.ID, "Payment approved by gateway")

		payment.Status = models.StatusSuccess
		payment.TransactionID = utils.GenerateTransactionID()
		now := time.Now()
		payment.ProcessedAt = &now
		payment.UpdatedAt = now

		s.store.SavePayment(payment)
		s.log.LogPayment("COMPLETED", payment.ID, fmt.Sprintf("Payment completed with transaction ID: %s", payment.TransactionID))
		s.publishPaymentEvent("payment.success", payment)
	} else {
		s.log.LogPayment("DECLINED", payment.ID, "Payment declined by gateway")
		s.updatePaymentStatus(payment, models.StatusFailed, "Payment declined by bank")
		s.publishPaymentEvent("payment.failed", payment)
	}
}

func (s *PaymentService) GetPayment(ctx context.Context, paymentID string) (*models.Payment, error) {
	s.log.LogPayment("LOOKUP", paymentID, "Retrieving payment details")

	payment, err := s.store.GetPayment(paymentID)
	if err != nil {
		s.log.LogPayment("NOT_FOUND", paymentID, "Payment not found in storage")
		return nil, ErrPaymentNotFound
	}

	s.log.LogPayment("FOUND", paymentID, fmt.Sprintf("Payment retrieved with status: %s", payment.Status))
	return payment, nil
}

func (s *PaymentService) RefundPayment(ctx context.Context, paymentID string, amount *float64, reason string) (*models.Payment, error) {
	s.log.LogPayment("REFUND_INIT", paymentID, fmt.Sprintf("Initiating refund, reason: %s", reason))

	payment, err := s.store.GetPayment(paymentID)
	if err != nil {
		s.log.LogPayment("REFUND_FAILED", paymentID, "Payment not found for refund")
		return nil, ErrPaymentNotFound
	}

	if payment.Status != models.StatusSuccess {
		s.log.LogPayment("REFUND_FAILED", paymentID, fmt.Sprintf("Payment not refundable, current status: %s", payment.Status))
		return nil, ErrPaymentNotRefundable
	}

	refundAmount := payment.Amount
	if amount != nil {
		if *amount <= 0 || *amount > payment.Amount {
			s.log.LogPayment("REFUND_FAILED", paymentID, fmt.Sprintf("Invalid refund amount: %.2f", *amount))
			return nil, ErrInvalidRefundAmount
		}
		refundAmount = *amount
	}

	s.log.LogPayment("REFUND_PROCESSING", paymentID, fmt.Sprintf("Processing refund of %.2f %s", refundAmount, payment.Currency))

	// Process refund
	payment.Status = models.StatusRefunded
	payment.UpdatedAt = time.Now()

	if err := s.store.SavePayment(payment); err != nil {
		s.log.Error("PAYMENT", fmt.Sprintf("Failed to save refund for payment %s: %v", paymentID, err))
		return nil, fmt.Errorf("failed to save refund: %w", err)
	}

	s.log.LogPayment("REFUND_SUCCESS", paymentID, fmt.Sprintf("Refund completed successfully: %.2f %s", refundAmount, payment.Currency))

	// Publish refund event
	s.publishPaymentEvent("payment.refunded", payment)

	return payment, nil
}

func (s *PaymentService) ProcessPaymentEvent(event *models.PaymentEvent) error {
	s.log.LogKafka("EVENT_RECEIVED", "payment-events", fmt.Sprintf("Processing event type: %s for payment: %s", event.Type, event.PaymentID))

	// Handle incoming payment events from Kafka
	switch event.Type {
	case "payment.created":
		return s.handlePaymentCreated(event.Payment)
	case "payment.webhook":
		return s.handleWebhookEvent(event.Payment)
	default:
		s.log.Warn("KAFKA", fmt.Sprintf("Unknown event type received: %s", event.Type))
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
}

func (s *PaymentService) handlePaymentCreated(payment *models.Payment) error {
	s.log.LogProcess("EVENT_HANDLER", fmt.Sprintf("Handling payment created event for: %s", payment.ID))
	// Process payment created event
	// This could trigger additional processing, notifications, etc.
	return nil
}

func (s *PaymentService) handleWebhookEvent(payment *models.Payment) error {
	s.log.LogProcess("WEBHOOK", fmt.Sprintf("Handling webhook event for payment: %s", payment.ID))
	// Handle webhook events from external payment processors
	return nil
}

func (s *PaymentService) validateCard(req *models.PaymentRequest) error {
	s.log.Debug("VALIDATION", fmt.Sprintf("Validating card ending in: %s", req.CardNumber[len(req.CardNumber)-4:]))

	// Simulate card validation
	if len(req.CardNumber) < 13 || len(req.CardNumber) > 19 {
		return ErrInvalidCard
	}

	// Check expiry date
	now := time.Now()
	if req.ExpiryYear < now.Year() || (req.ExpiryYear == now.Year() && req.ExpiryMonth < int(now.Month())) {
		return ErrCardExpired
	}

	// Simulate random card validation failures
	if rand.Float32() < 0.05 { // 5% chance of invalid card
		return ErrInvalidCard
	}

	return nil
}

func (s *PaymentService) shouldPaymentSucceed(req *models.PaymentRequest) bool {
	// Simulate payment success/failure based on various factors

	// Always fail for specific test card numbers
	testFailCards := []string{"4000000000000002", "4000000000000010"}
	for _, card := range testFailCards {
		if req.CardNumber == card {
			s.log.LogPayment("TEST_CARD", "test", fmt.Sprintf("Using test failure card: %s", card))
			return false
		}
	}

	// Simulate insufficient funds for large amounts
	if req.Amount > 10000 {
		success := rand.Float32() > 0.3 // 70% failure rate for large amounts
		s.log.LogPayment("LARGE_AMOUNT", "test", fmt.Sprintf("Large amount %.2f, success: %t", req.Amount, success))
		return success
	}

	// General success rate of 95%
	success := rand.Float32() > 0.05
	s.log.Debug("PAYMENT", fmt.Sprintf("Payment simulation result: %t", success))
	return success
}

func (s *PaymentService) updatePaymentStatus(payment *models.Payment, status models.PaymentStatus, errorMsg string) {
	s.log.LogPayment("STATUS_UPDATE", payment.ID, fmt.Sprintf("Updating status from %s to %s", payment.Status, status))

	payment.Status = status
	payment.UpdatedAt = time.Now()
	if errorMsg != "" {
		payment.ErrorMessage = errorMsg
		s.log.LogPayment("ERROR_SET", payment.ID, fmt.Sprintf("Error message: %s", errorMsg))
	}
	s.store.SavePayment(payment)
}

func (s *PaymentService) publishPaymentEvent(eventType string, payment *models.Payment) {
	s.log.LogKafka("PUBLISH", "payment-events", fmt.Sprintf("Publishing %s event for payment %s", eventType, payment.ID))

	event := &models.PaymentEvent{
		Type:      eventType,
		PaymentID: payment.ID,
		Payment:   payment,
		Timestamp: time.Now(),
	}

	if err := s.producer.PublishPaymentEvent(event); err != nil {
		s.log.Error("KAFKA", fmt.Sprintf("Failed to publish payment event %s for payment %s: %v", eventType, payment.ID, err))
		// In production, you might want to implement retry logic or dead letter queue
		s.log.LogProcess("FALLBACK", fmt.Sprintf("Payment %s processed successfully despite Kafka publish failure", payment.ID))
	} else {
		s.log.LogKafka("PUBLISHED", "payment-events", fmt.Sprintf("Successfully published %s event for payment %s", eventType, payment.ID))
	}
}

func (s *PaymentService) maskCardNumber(cardNumber string) string {
	if len(cardNumber) < 4 {
		return cardNumber
	}
	return "****-****-****-" + cardNumber[len(cardNumber)-4:]
}
