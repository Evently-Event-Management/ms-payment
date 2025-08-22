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
	s.log.LogPayment("INIT", "new", fmt.Sprintf("Processing payment for order %s, price: %.2f",
		req.OrderID, req.Price))

	otp, _ := otp2.GenerateOTP()
	otp2.SendEmailOTP("isurumuni.22@cse.mrt.ac.lk", otp)
	ok, err := s.redis.AddOTP(otp, req.OrderID)
	if err != nil {
		fmt.Printf("Error locking OTP: %v\n", err)
		return nil, fmt.Errorf("redis error: %w", err)
	}
	if !ok {
		fmt.Println("OTP already locked for this order. Aborting payment.")
		return nil, fmt.Errorf("OTP already locked")
	}

	// Create payment record
	payment := &models.Payment{
		PaymentID: req.PaymentID,
		OrderID:   req.OrderID,
		Status:    models.StatusPending,
		Price:     req.Price,
		Date:      time.Now(),
	}

	s.log.LogPayment("CREATE", payment.PaymentID, fmt.Sprintf("Payment record created with status: %s", payment.Status))

	// Save payment to storage
	if err := s.store.SavePayment(payment); err != nil {
		s.log.Error("PAYMENT", fmt.Sprintf("Failed to save payment %s: %v", payment.PaymentID, err))
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	s.log.LogDatabase("SAVE", "payments", fmt.Sprintf("Payment %s saved successfully", payment.PaymentID))

	// Simulate payment processing
	go s.processPaymentAsync(ctx, payment, req)

	s.log.LogPayment("ASYNC", payment.PaymentID, "Payment processing started asynchronously")
	return payment, nil
}

func (s *PaymentService) processPaymentAsync(ctx context.Context, payment *models.Payment, req *models.PaymentRequest) {
	s.log.LogProcess("ASYNC_PAYMENT", fmt.Sprintf("Starting async processing for payment %s", payment.PaymentID))

	// Simulate processing delay
	processingTime := time.Duration(rand.Intn(3)+1) * time.Second
	s.log.LogPayment("PROCESSING", payment.PaymentID, fmt.Sprintf("Simulating processing delay: %v", processingTime))
	time.Sleep(processingTime)

	// Simulate payment gateway response
	if s.shouldPaymentSucceed(req) {
		s.log.LogPayment("SUCCESS", payment.PaymentID, "Payment approved by gateway")

		payment.Status = models.StatusSuccess
		payment.Date = time.Now()

		s.store.SavePayment(payment)

	} else {
		s.log.LogPayment("DECLINED", payment.PaymentID, "Payment declined by gateway")
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

	if amount != nil {
		if *amount <= 0 || *amount > payment.Price {
			s.log.LogPayment("REFUND_FAILED", paymentID, fmt.Sprintf("Invalid refund amount: %.2f", *amount))
			return nil, ErrInvalidRefundAmount
		}
	}

	s.log.LogPayment("REFUND_PROCESSING", paymentID, fmt.Sprintf("Processing refund of %.2f", payment.Price))

	// Process refund
	payment.Status = models.StatusRefunded
	payment.Date = time.Now()

	if err := s.store.SavePayment(payment); err != nil {
		s.log.Error("PAYMENT", fmt.Sprintf("Failed to save refund for payment %s: %v", paymentID, err))
		return nil, fmt.Errorf("failed to save refund: %w", err)
	}

	s.log.LogPayment("REFUND_SUCCESS", paymentID, "Refund completed successfully")

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
	s.log.LogProcess("EVENT_HANDLER", fmt.Sprintf("Handling payment created event for: %s", payment.PaymentID))
	// Process payment created event
	return nil
}

func (s *PaymentService) handleWebhookEvent(payment *models.Payment) error {
	s.log.LogProcess("WEBHOOK", fmt.Sprintf("Handling webhook event for payment: %s", payment.PaymentID))
	// Handle webhook events from external payment processors
	return nil
}

func (s *PaymentService) shouldPaymentSucceed(req *models.PaymentRequest) bool {
	// Simulate payment success/failure
	if req.Price > 10000 {
		success := rand.Float32() > 0.3 // 70% failure rate for large amounts
		s.log.LogPayment("LARGE_AMOUNT", "test", fmt.Sprintf("Large amount %.2f, success: %t", req.Price, success))
		return success
	}
	// General success rate of 95%
	success := rand.Float32() > 0.05
	s.log.Debug("PAYMENT", fmt.Sprintf("Payment simulation result: %t", success))
	return success
}

func (s *PaymentService) updatePaymentStatus(payment *models.Payment, status models.PaymentStatus, errorMsg string) {
	s.log.LogPayment("STATUS_UPDATE", payment.PaymentID, fmt.Sprintf("Updating status from %s to %s", payment.Status, status))

	payment.Status = status
	payment.Date = time.Now()
	s.store.SavePayment(payment)
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

func (s *PaymentService) maskCardNumber(cardNumber string) string {
	if len(cardNumber) < 4 {
		return cardNumber
	}
	return "****-****-****-" + cardNumber[len(cardNumber)-4:]
}

func (s *PaymentService) VerifyOTP(orderID string, otp string) bool {
	s.log.Debug("OTP", fmt.Sprintf("🔍 Starting OTP verification for orderID=%s with provided OTP=%s", orderID, otp))
	payment, err := s.store.GetTicketByOrderID(orderID)
	if err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Failed to get payment for order %s: %v", orderID, err))
		s.publishPaymentEvent("otp.failed", payment)
		_ = s.redis.RemoveOTP(orderID)
		return false
	}
	lockedOTP, err := s.redis.GetOTP(orderID)
	if err != nil {
		s.log.Error("OTP", fmt.Sprintf("❌ Error fetching OTP for order %s: %v", orderID, err))
		s.handleOTPFail(orderID)
		s.publishPaymentEvent("otp.failed", payment)
		_ = s.redis.RemoveOTP(orderID)
		return false
	}

	s.log.Debug("OTP", fmt.Sprintf("📦 Redis returned OTP for orderID=%s: %s", orderID, lockedOTP))

	if lockedOTP == "" {
		s.log.Warn("OTP", fmt.Sprintf("⚠️ No OTP found in Redis for order %s", orderID))
		s.publishPaymentEvent("otp.failed", payment)
		s.handleOTPFail(orderID)
		_ = s.redis.RemoveOTP(orderID)
		return false
	}

	if lockedOTP == otp {
		s.log.Info("OTP", fmt.Sprintf("✅ OTP matched for order %s. Provided=%s, Stored=%s", orderID, otp, lockedOTP))

		payment.Status = models.StatusSuccess
		err = s.store.UpdatePayment(payment)
		if err != nil {
			s.log.Error("DATABASE", fmt.Sprintf("Failed to update payment for order %s: %v", orderID, err))
			s.handleOTPFail(orderID)
			s.publishPaymentEvent("otp.failed", payment)
			_ = s.redis.RemoveOTP(orderID)
			return false
		}
		s.log.Debug("OTP", fmt.Sprintf("🗑 OTP removed from Redis for order %s", orderID))
		s.publishPaymentEvent("otp.success", payment)
		_ = s.redis.RemoveOTP(orderID)
		return true
	}

	s.log.Warn("OTP", fmt.Sprintf("🚫 OTP mismatch for order %s. Provided=%s, Stored=%s", orderID, otp, lockedOTP))
	s.handleOTPFail(orderID)
	s.publishPaymentEvent("otp.failed", payment)
	_ = s.redis.RemoveOTP(orderID)
	return false
}

func (s *PaymentService) handleOTPFail(orderID string) {
	payment, err := s.store.GetTicketByOrderID(orderID)
	if err == nil && payment != nil {
		payment.Status = models.StatusFailed
		_ = s.store.UpdatePayment(payment)
	}
}
