package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"payment-gateway/internal/kafka"
	"payment-gateway/internal/models"
	"payment-gateway/internal/services"
	"payment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
)

type StripeHandler struct {
	stripeService  *services.StripeService
	paymentService *services.PaymentService
	producer       *kafka.Producer
}

func NewStripeHandler(stripeService *services.StripeService, paymentService *services.PaymentService, producer *kafka.Producer) *StripeHandler {
	return &StripeHandler{
		stripeService:  stripeService,
		paymentService: paymentService,
		producer:       producer,
	}
}

// ValidateCard validates credit card details without creating a charge
func (h *StripeHandler) ValidateCard(c *gin.Context) {
	var req models.StripeCardValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", err.Error()))
		return
	}

	// SECURITY ENHANCEMENT: Verify the order exists in our database
	// This ensures we only validate cards for legitimate orders
	_, err := h.paymentService.GetPaymentByOrderID(c.Request.Context(), req.OrderID)
	if err != nil {
		log.Printf("No existing payment found for order %s during card validation", req.OrderID)
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request",
			"No payment record found for this order_id. Create a payment record first."))
		return
	}

	// Map StripeCardDetails to StripeCard
	card := &models.StripeCard{
		Number:   req.Card.Number,
		ExpMonth: req.Card.ExpMonth,
		ExpYear:  req.Card.ExpYear,
		CVC:      req.Card.CVC,
		Name:     req.Card.Name,
	}
	result, err := h.stripeService.ValidateCard(card)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Card validation failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Card validation result", result))
}

// ProcessPayment processes a payment through Stripe
func (h *StripeHandler) ProcessPayment(c *gin.Context) {
	var req models.StripePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", err.Error()))
		return
	}

	// Validate order_id is provided
	if req.OrderID == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", "order_id is required"))
		return
	}

	// Set default currency if not provided
	if req.Currency == "" {
		req.Currency = "usd"
	}

	// Validate token or card is provided
	if req.Token == "" && req.Card == nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", "Either token or card must be provided"))
		return
	}

	// SECURITY ENHANCEMENT: Always fetch payment details from database using order_id
	// This prevents the frontend from specifying the amount, which could be a security risk
	existingPayment, err := h.paymentService.GetPaymentByOrderID(c.Request.Context(), req.OrderID)
	if err != nil {
		log.Printf("No existing payment found for order %s", req.OrderID)
		// Check if this is for a new order with no payment record yet
		// In that case, we should return an error as we can't proceed without knowing the amount
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request",
			"No payment record found for this order_id. Create a payment record first."))
		return
	}

	// Override any amount provided in the request with the amount from the database
	req.Amount = existingPayment.Price
	log.Printf("Using price %.2f from database for order %s", req.Amount, req.OrderID)

	// Also get the payment ID if available and add it to the request
	if existingPayment.PaymentID != "" {
		req.PaymentID = existingPayment.PaymentID
		log.Printf("Using existing payment ID %s for order %s", req.PaymentID, req.OrderID)
	}

	// Process payment through Stripe
	result, err := h.stripeService.ProcessPayment(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Payment processing failed", err.Error()))
		return
	}

	// Update existing payment record with results from Stripe
	if result.Status == models.StatusSuccess || result.Status == models.StatusPending {
		// We already have the existing payment from earlier database lookup
		paymentReq := &models.PaymentRequest{
			OrderID: result.OrderID,
			Status:  result.Status,
			// Price already set in the database, no need to update it
			URL:    result.ReceiptURL, // Use receipt URL if available
			Source: "stripe",          // Mark this as a Stripe payment to skip OTP
		}

		// If receipt URL is empty, use a default URL
		if paymentReq.URL == "" {
			paymentReq.URL = fmt.Sprintf("https://payment.gateway.com/checkout/%s", result.OrderID)
		}

		// If we already have a payment ID, include it in the request
		if req.PaymentID != "" {
			paymentReq.PaymentID = req.PaymentID
		}

		paymentRecord, err := h.paymentService.ProcessPayment(c.Request.Context(), paymentReq)
		if err != nil {
			// Log the error but continue since the Stripe payment was successful
			log.Printf("Failed to update payment record: %v", err)
		}

		// Update the payment record with the transaction ID if available
		if result.TransactionID != "" && paymentRecord != nil && paymentRecord.TransactionID == "" {
			paymentRecord.TransactionID = result.TransactionID
			log.Printf("Updating payment %s with transaction ID %s", paymentRecord.PaymentID, result.TransactionID)

			// Create a new payment request with updated details to process
			updatedPaymentReq := &models.PaymentRequest{
				PaymentID: paymentRecord.PaymentID, // Use the payment ID from the record
				OrderID:   paymentRecord.OrderID,
				Status:    paymentRecord.Status,
				URL:       paymentRecord.URL,
				Source:    "stripe", // Ensure source is set for updates too
			}

			// Re-process the payment to update it
			_, updateErr := h.paymentService.ProcessPayment(c.Request.Context(), updatedPaymentReq)
			if updateErr != nil {
				log.Printf("Failed to update payment record with transaction ID: %v", updateErr)
			}
		}

		// Return both Stripe result and our payment record
		response := map[string]interface{}{
			"stripe_result":  result,
			"payment_record": paymentRecord,
		}

		// Also stream the payment event to Kafka if payment was successful
		if result.Status == models.StatusSuccess {
			event := &models.PaymentEvent{
				Type:      "payment.success",
				PaymentID: paymentRecord.PaymentID,
				Payment:   paymentRecord,
				Timestamp: time.Now(),
			}

			if err := h.producer.PublishPaymentEvent(event); err != nil {
				log.Printf("Warning: Failed to publish success event to Kafka: %v", err)
			} else {
				log.Printf("Payment success event published to Kafka for payment %s", paymentRecord.PaymentID)
			}
		} else if result.Status == models.StatusFailed {
			event := &models.PaymentEvent{
				Type:      "payment.failed",
				PaymentID: paymentRecord.PaymentID,
				Payment:   paymentRecord,
				Timestamp: time.Now(),
			}

			if err := h.producer.PublishPaymentEvent(event); err != nil {
				log.Printf("Warning: Failed to publish failure event to Kafka: %v", err)
			} else {
				log.Printf("Payment failure event published to Kafka for payment %s", paymentRecord.PaymentID)
			}
		}

		c.JSON(http.StatusOK, utils.SuccessResponse("Payment processed", response))
		return
	}

	// When we don't have a payment record, still publish to Kafka based on the Stripe result
	event := &models.PaymentEvent{
		Type:      "payment." + string(result.Status),
		PaymentID: result.TransactionID, // Use transaction ID as payment ID in this case
		Payment:   nil,                  // No payment record available
		Timestamp: time.Now(),
	}

	if err := h.producer.PublishPaymentEvent(event); err != nil {
		log.Printf("Warning: Failed to publish event to Kafka: %v", err)
	} else {
		log.Printf("Payment event published to Kafka for transaction %s with status %s",
			result.TransactionID, result.Status)
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment processed", result))
}

// RefundPayment refunds a payment through Stripe
func (h *StripeHandler) RefundPayment(c *gin.Context) {
	var req models.StripeRefundRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", err.Error()))
		return
	}

	// Security enhancement: Fetch payment details from database using order_id
	existingPayment, err := h.paymentService.GetPaymentByOrderID(c.Request.Context(), req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request",
			"No payment record found for this order_id"))
		return
	}

	// Ensure the payment is in a state that can be refunded
	if existingPayment.Status != models.StatusSuccess {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request",
			fmt.Sprintf("Payment with status %s cannot be refunded", existingPayment.Status)))
		return
	}

	// Use the order_id and reason from the request, no need for amount as it will be fetched from DB
	// Store the payment ID in a variable to pass to the service
	paymentID := existingPayment.PaymentID

	// Create a StripeRefundRequest for the stripeService
	stripeReq := &models.StripeRefundRequest{
		OrderID: req.OrderID,
		Reason:  req.Reason,
	}

	// Process refund through Stripe
	refundedPayment, err := h.stripeService.RefundPayment(c.Request.Context(), stripeReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Refund processing failed", err.Error()))
		return
	}

	// Update the payment in the database
	if err := h.paymentService.UpdatePaymentStatus(c.Request.Context(), paymentID, models.StatusRefunded); err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to update payment status", err.Error()))
		return
	}

	// Publish the refund event to Kafka
	h.publishRefundEvent(refundedPayment)

	log.Printf("Payment refund processed for order %s, payment %s", req.OrderID, paymentID)

	c.JSON(http.StatusOK, utils.SuccessResponse("Refund processed", refundedPayment))
}

// GetPaymentDetails retrieves payment details from Stripe
func (h *StripeHandler) GetPaymentDetails(c *gin.Context) {
	paymentID := c.Param("id")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Payment ID is required", ""))
		return
	}

	// Get payment details from Stripe
	result, err := h.stripeService.GetPaymentDetails(c.Request.Context(), paymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to retrieve payment details", err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment details retrieved", result))
}

// HandleStripeWebhook handles webhook events from Stripe
func (h *StripeHandler) HandleStripeWebhook(c *gin.Context) {
	// Read the request body
	_, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to read request body", err.Error()))
		return
	}

	// Get the Stripe-Signature header
	// stripeSignature := c.GetHeader("Stripe-Signature")

	// This is just a placeholder for webhook handling
	// In a real application, you would verify the signature and process the event
	c.JSON(http.StatusOK, gin.H{"received": true})
}

// StreamPaymentToKafka streams payment events to Kafka
func (h *StripeHandler) StreamPaymentToKafka(c *gin.Context) {
	var req models.PaymentStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", err.Error()))
		return
	}

	// Validate payment_id is provided
	if req.PaymentID == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", "payment_id is required"))
		return
	}

	// Get payment details from our database
	payment, err := h.paymentService.GetPayment(c.Request.Context(), req.PaymentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to retrieve payment details", err.Error()))
		return
	}

	// Create payment event
	eventType := "payment.event"
	if req.Status == "success" || payment.Status == models.StatusSuccess {
		eventType = "payment.success"
	} else if req.Status == "failed" || payment.Status == models.StatusFailed {
		eventType = "payment.failed"
	} else if req.Status == "refunded" || payment.Status == models.StatusRefunded {
		eventType = "payment.refunded"
	}

	event := &models.PaymentEvent{
		Type:      eventType,
		PaymentID: payment.PaymentID,
		Payment:   payment,
		Timestamp: time.Now(),
	}

	// Publish event to Kafka
	if err := h.producer.PublishPaymentEvent(event); err != nil {
		log.Printf("Failed to publish payment event to Kafka: %v", err)
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to stream payment event", err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment event streamed successfully", map[string]interface{}{
		"event_type": eventType,
		"payment_id": payment.PaymentID,
		"status":     payment.Status,
	}))
}

// publishRefundEvent publishes a refund event to Kafka
func (h *StripeHandler) publishRefundEvent(payment *models.Payment) {
	// Create event for Kafka
	event := &models.PaymentEvent{
		Type:      "payment.refunded",
		PaymentID: payment.PaymentID,
		OrderID:   payment.OrderID,
		Payment:   payment,
		Timestamp: time.Now(),
	}

	// Publish to Kafka
	if err := h.producer.PublishPaymentEvent(event); err != nil {
		log.Printf("Failed to publish refund event: %v", err)
	} else {
		log.Printf("Published refund event for payment %s", payment.PaymentID)
	}
}
