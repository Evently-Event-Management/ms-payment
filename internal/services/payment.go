package services

import (
	"context"
	"errors"
	"fmt"
	otp2 "payment-gateway/internal/otp"
	"payment-gateway/internal/utils"
	"time"

	"payment-gateway/internal/kafka"
	"payment-gateway/internal/logger"
	"payment-gateway/internal/models"
	"payment-gateway/internal/storage"
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

type RedisLock interface {
	AddOTP(otp, orderID string) (bool, error)
	RemoveOTP(orderID string) error
	IsOTPLocked(orderID string) (bool, error)
	GetOTP(orderID string) (string, error)
}
type PaymentService struct {
	store    storage.Store
	producer *kafka.Producer
	log      *logger.Logger
	redis    RedisLock // Added logger to service
}

func NewPaymentService(store storage.Store, producer *kafka.Producer, log *logger.Logger, redis RedisLock) *PaymentService {
	return &PaymentService{
		store:    store,
		producer: producer,
		log:      log,
		redis:    redis,
	}
}

func (s *PaymentService) OtpSender(email string) {

	// Simulate sending OTP
	otp, _ := otp2.GenerateOTP()
	otp2.SendEmailOTP(email, otp)

	s.log.Info("OTP", fmt.Sprintf("Sent OTP to %s: %s", email, otp))

}
func (s *PaymentService) ProcessPayment(ctx context.Context, req *models.PaymentRequest) (*models.Payment, error) {
	s.log.LogPayment("INIT", "new", fmt.Sprintf("Processing payment for order %s",
		req.OrderID))

	var existingPayment *models.Payment
	var err error

	// If PaymentID is provided, try to get the payment by ID first
	if req.PaymentID != "" {
		existingPayment, err = s.store.GetPayment(req.PaymentID)
		if err == nil && existingPayment != nil {
			s.log.LogPayment("EXISTING", existingPayment.PaymentID, fmt.Sprintf("Found existing payment by ID %s", req.PaymentID))
		} else {
			s.log.LogPayment("NOT_FOUND", req.PaymentID, "Payment ID provided but not found, will try order ID lookup")
		}
	}

	// If we couldn't find by PaymentID or it wasn't provided, check by OrderID
	if existingPayment == nil {
		existingPayment, err = s.store.GetTicketByOrderID(req.OrderID)
		if err == nil && existingPayment != nil {
			s.log.LogPayment("EXISTING", existingPayment.PaymentID, fmt.Sprintf("Found existing payment for order %s", req.OrderID))
		}
	}

	// If we found an existing payment, update it
	if existingPayment != nil {
		// Update existing payment with new status and data
		existingPayment.Status = req.Status
		existingPayment.UpdatedDate = time.Now()

		// Only update price if provided
		if req.Price > 0 {
			existingPayment.Price = req.Price
		}

		// Update URL if provided
		if req.URL != "" {
			existingPayment.URL = req.URL
		}

		if req.Source != "" {
			s.log.LogPayment("SOURCE", existingPayment.PaymentID, fmt.Sprintf("Payment source: %s", req.Source))
		}

		// Update the payment in storage
		if err := s.store.UpdatePayment(existingPayment); err != nil {
			s.log.Error("PAYMENT", fmt.Sprintf("Failed to update existing payment %s: %v", existingPayment.PaymentID, err))
			return nil, fmt.Errorf("failed to update payment: %w", err)
		}

		s.log.LogPayment("UPDATE", existingPayment.PaymentID, fmt.Sprintf("Updated payment status to %s", existingPayment.Status))

		// Publish event based on status
		switch existingPayment.Status {
		case models.StatusSuccess:
			s.publishPaymentEvent("payment.success", existingPayment)
		case models.StatusFailed:
			s.publishPaymentEvent("payment.failed", existingPayment)
		}

		return existingPayment, nil
	}

	// If no existing payment found, create a new one
	now := time.Now()

	// Generate payment ID or use the one provided in the request
	var paymentID string
	if req.PaymentID != "" {
		paymentID = req.PaymentID
		s.log.LogPayment("CREATE", paymentID, "Using payment ID from request")
	} else {
		paymentID = fmt.Sprintf("pay_%d", time.Now().UnixNano())
		s.log.LogPayment("CREATE", paymentID, "Generated new payment ID")
	}

	// Create new payment record
	payment := &models.Payment{
		PaymentID:   paymentID,
		OrderID:     req.OrderID,
		Status:      req.Status,
		CreatedDate: now,
		UpdatedDate: now,
	}

	// Only set price if provided
	if req.Price > 0 {
		payment.Price = req.Price
		s.log.LogPayment("CREATE", payment.PaymentID, fmt.Sprintf("Payment record created with price: %.2f and status: %s",
			payment.Price, payment.Status))
	} else {
		s.log.LogPayment("CREATE", payment.PaymentID, fmt.Sprintf("Payment record created with status: %s (no price provided)",
			payment.Status))
	}

	// Set URL if provided
	if req.URL != "" {
		payment.URL = req.URL
	}

	// Save payment to storage
	if err := s.store.SavePayment(payment); err != nil {
		s.log.Error("PAYMENT", fmt.Sprintf("Failed to save payment %s: %v", payment.PaymentID, err))
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	s.log.LogDatabase("SAVE", "payments", fmt.Sprintf("Payment %s saved successfully", payment.PaymentID))

	// Publish event based on status
	if payment.Status == models.StatusSuccess {
		s.publishPaymentEvent("payment.success", payment)
	} else if payment.Status == models.StatusFailed {
		s.publishPaymentEvent("payment.failed", payment)
	}

	return payment, nil
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

// GetPaymentByOrderID retrieves a payment by order ID
func (s *PaymentService) GetPaymentByOrderID(ctx context.Context, orderID string) (*models.Payment, error) {
	s.log.LogPayment("LOOKUP_BY_ORDER", orderID, "Retrieving payment details by order ID")

	payment, err := s.store.GetTicketByOrderID(orderID)
	if err != nil {
		s.log.LogPayment("NOT_FOUND", orderID, "Payment not found for order ID")
		return nil, ErrPaymentNotFound
	}

	s.log.LogPayment("FOUND", payment.PaymentID, fmt.Sprintf("Payment retrieved for order %s with status: %s",
		orderID, payment.Status))
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

	if amount != nil {
		if *amount <= 0 || *amount > payment.Price {
			s.log.LogPayment("REFUND_FAILED", paymentID, fmt.Sprintf("Invalid refund amount: %.2f", *amount))
			return nil, ErrInvalidRefundAmount
		}
	}

	s.log.LogPayment("REFUND_PROCESSING", paymentID, fmt.Sprintf("Processing refund of %.2f", payment.Price))

	// Process refund
	payment.Status = models.StatusRefunded
	payment.UpdatedDate = time.Now()

	if err := s.store.UpdatePayment(payment); err != nil {
		s.log.Error("PAYMENT", fmt.Sprintf("Failed to update refund status for payment %s: %v", paymentID, err))
		return nil, fmt.Errorf("failed to save refund: %w", err)
	}

	s.log.LogPayment("REFUND_SUCCESS", paymentID, "Refund completed successfully")

	// Publish refund event to Kafka
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
	s.log.LogProcess("EVENT_HANDLER", fmt.Sprintf("Handling payment created event for: %s", payment.PaymentID))
	// Process payment created event
	return nil
}

func (s *PaymentService) handleWebhookEvent(payment *models.Payment) error {
	s.log.LogProcess("WEBHOOK", fmt.Sprintf("Handling webhook event for payment: %s", payment.PaymentID))
	// Handle webhook events from external payment processors
	return nil
}
func (s *PaymentService) ProcessOrderEvent(order *models.Order) error {
	s.log.LogKafka("ORDER_RECEIVED", "order.created", fmt.Sprintf("Processing order: %s with status: %s", order.OrderID, order.Status))

	// Check if a payment already exists for this order
	payment, err := s.store.GetTicketByOrderID(order.OrderID)
	if err == nil && payment != nil {
		s.log.Warn("KAFKA", fmt.Sprintf("Payment already exists for order %s, skipping", order.OrderID))
		return nil
	}

	// Create a new payment record for this order
	payment = &models.Payment{
		PaymentID:   utils.GenerateUUID(),
		OrderID:     order.OrderID,
		Status:      models.StatusPending,
		Price:       order.Price,
		CreatedDate: time.Now(),
		URL:         fmt.Sprintf("https://payment.gateway.com/checkout/%s", order.OrderID),
	}

	// Save the payment to the database
	if err := s.store.SavePayment(payment); err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Failed to save payment for order %s: %v", order.OrderID, err))
		return fmt.Errorf("failed to save payment: %w", err)
	}

	s.log.LogDatabase("SAVE", "payments", fmt.Sprintf("Payment %s created for order %s", payment.PaymentID, order.OrderID))

	// Publish a payment.created event
	s.publishPaymentEvent("payment.created", payment)

	return nil
}

func (s *PaymentService) publishPaymentEvent(eventType string, payment *models.Payment) {
	s.log.LogKafka("PUBLISH", "payment-events", fmt.Sprintf("Publishing %s event for payment %s", eventType, payment.PaymentID))

	event := &models.PaymentEvent{
		Type:      eventType,
		PaymentID: payment.PaymentID,
		Payment:   payment,
		Timestamp: time.Now(),
	}

	if err := s.producer.PublishPaymentEvent(event); err != nil {
		s.log.Error("KAFKA", fmt.Sprintf("Failed to publish payment event %s for payment %s: %v", eventType, payment.PaymentID, err))
		s.log.LogProcess("FALLBACK", fmt.Sprintf("Payment %s processed successfully despite Kafka publish failure", payment.PaymentID))
	} else {
		s.log.LogKafka("PUBLISHED", "payment-events", fmt.Sprintf("Successfully published %s event for payment %s", eventType, payment.PaymentID))
	}
}

// UpdatePaymentStatus updates the status of a payment in the database and returns the updated payment
func (s *PaymentService) UpdatePaymentStatus(ctx context.Context, paymentID string, status models.PaymentStatus) error {
	s.log.LogPayment("UPDATE", paymentID, fmt.Sprintf("Updating payment status to %s", status))

	// Get the existing payment
	payment, err := s.store.GetPayment(paymentID)
	if err != nil {
		s.log.Error("PAYMENT", fmt.Sprintf("Failed to get payment for status update: %v", err))
		return fmt.Errorf("failed to get payment: %w", err)
	}

	// Update the payment status
	payment.Status = status
	payment.UpdatedDate = time.Now()

	// Save the updated payment
	if err := s.store.UpdatePayment(payment); err != nil {
		s.log.Error("PAYMENT", fmt.Sprintf("Failed to update payment status: %v", err))
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	s.log.LogPayment("UPDATE_SUCCESS", paymentID, fmt.Sprintf("Payment status updated to %s", status))
	return nil
}
