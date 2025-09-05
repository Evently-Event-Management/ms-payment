package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"payment-gateway/internal/logger"
	"payment-gateway/internal/models"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/client"
)

var (
	ErrStripeAPIError         = errors.New("stripe API error")
	ErrStripeClientInitFailed = errors.New("failed to initialize Stripe client")
	ErrCardValidationFailed   = errors.New("card validation failed")
)

// StripeService handles integration with Stripe payment gateway
type StripeService struct {
	client *client.API
	log    *logger.Logger
}

// parseStringToInt64 safely converts a string to int64, returns 0 if conversion fails
func parseStringToInt64(s string) int64 {
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// NewStripeService creates a new instance of StripeService
func NewStripeService(log *logger.Logger) (*StripeService, error) {
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		log.Error("STRIPE", "STRIPE_SECRET_KEY environment variable not set")
		return nil, ErrStripeClientInitFailed
	}

	sc := client.New(stripeKey, nil)
	if sc == nil {
		log.Error("STRIPE", "Failed to initialize Stripe client")
		return nil, ErrStripeClientInitFailed
	}

	log.Info("STRIPE", "Stripe client initialized successfully")
	return &StripeService{
		client: sc,
		log:    log,
	}, nil
}

// ValidateCard validates the provided card details using Stripe
func (s *StripeService) ValidateCard(card *models.StripeCard) (*models.StripeCardValidationResponse, error) {
	// Create a payment method to validate the card
	params := &stripe.PaymentMethodParams{
		Type: stripe.String("card"),
		Card: &stripe.PaymentMethodCardParams{
			Number:   stripe.String(card.Number),
			ExpMonth: stripe.Int64(parseStringToInt64(card.ExpMonth)),
			ExpYear:  stripe.Int64(parseStringToInt64(card.ExpYear)),
			CVC:      stripe.String(card.CVC),
		},
	}

	pm, err := s.client.PaymentMethods.New(params)
	if err != nil {
		s.log.Error("STRIPE", fmt.Sprintf("Card validation failed: %v", err))

		return &models.StripeCardValidationResponse{
			Valid:   false,
			Message: err.Error(),
		}, nil
	}

	// If we get here, the card is valid
	response := &models.StripeCardValidationResponse{
		Valid:    true,
		Message:  "Card is valid",
		CardType: string(pm.Card.Brand),
		Last4:    pm.Card.Last4,
	}

	s.log.LogPayment("VALIDATE", "card", fmt.Sprintf("Card validation successful: %s ending in %s", response.CardType, response.Last4))

	// Clean up the payment method since we don't need it anymore
	_, err = s.client.PaymentMethods.Detach(pm.ID, &stripe.PaymentMethodDetachParams{})
	if err != nil {
		s.log.Warn("STRIPE", fmt.Sprintf("Failed to detach payment method: %v", err))
	}

	return response, nil
}

// ProcessPayment processes a payment through Stripe
func (s *StripeService) ProcessPayment(ctx context.Context, req *models.StripePaymentRequest) (*models.StripePaymentResponse, error) {
	paymentIdentifier := req.PaymentID
	if paymentIdentifier == "" {
		paymentIdentifier = "new"
	}

	s.log.LogPayment("PROCESS", paymentIdentifier, fmt.Sprintf("Processing Stripe payment for order %s, amount: %.2f %s",
		req.OrderID, req.Amount, req.Currency))

	// Validate that we have an amount to charge
	if req.Amount <= 0 {
		s.log.LogPayment("ERROR", paymentIdentifier, fmt.Sprintf("Invalid amount for order %s: %.2f", req.OrderID, req.Amount))
		return nil, fmt.Errorf("invalid payment amount: %.2f", req.Amount)
	}

	var paymentMethod string
	if req.Token != "" {
		paymentMethod = req.Token
		s.log.LogPayment("STRIPE", paymentIdentifier, "Using provided token/payment method ID")
	} else if req.Card != nil {
		// Legacy/test: create payment method from card
		pmParams := &stripe.PaymentMethodParams{
			Type: stripe.String("card"),
			Card: &stripe.PaymentMethodCardParams{
				Number:   stripe.String(req.Card.Number),
				ExpMonth: stripe.Int64(parseStringToInt64(req.Card.ExpMonth)),
				ExpYear:  stripe.Int64(parseStringToInt64(req.Card.ExpYear)),
				CVC:      stripe.String(req.Card.CVC),
			},
		}
		if req.Card.Name != "" {
			pmParams.BillingDetails = &stripe.PaymentMethodBillingDetailsParams{
				Name: stripe.String(req.Card.Name),
			}
			if req.Card.Address != nil {
				pmParams.BillingDetails.Address = &stripe.AddressParams{
					Line1:      stripe.String(req.Card.Address.Line1),
					Line2:      stripe.String(req.Card.Address.Line2),
					City:       stripe.String(req.Card.Address.City),
					State:      stripe.String(req.Card.Address.State),
					PostalCode: stripe.String(req.Card.Address.PostalCode),
					Country:    stripe.String(req.Card.Address.Country),
				}
			}
		}
		s.log.LogPayment("STRIPE", req.PaymentID, "Creating payment method from card")
		pm, err := s.client.PaymentMethods.New(pmParams)
		if err != nil {
			s.log.Error("STRIPE", fmt.Sprintf("Failed to create payment method: %v", err))
			return nil, fmt.Errorf("%w: %v", ErrStripeAPIError, err)
		}
		paymentMethod = pm.ID
		s.log.LogPayment("STRIPE", req.PaymentID, fmt.Sprintf("Payment method created: %s", pm.ID))
	} else {
		return nil, fmt.Errorf("%w: no payment method provided", ErrStripeAPIError)
	}

	// Convert amount to cents (Stripe uses smallest currency unit)
	amountInCents := int64(req.Amount * 100)
	metadata := make(map[string]string)
	metadata["payment_id"] = req.PaymentID
	metadata["order_id"] = req.OrderID

	// Add any additional metadata from the request
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	// Create a payment intent
	piParams := &stripe.PaymentIntentParams{
		Amount:             stripe.Int64(amountInCents),
		Currency:           stripe.String(req.Currency),
		PaymentMethod:      stripe.String(paymentMethod),
		Description:        stripe.String(req.Description),
		Metadata:           metadata,
		ConfirmationMethod: stripe.String("manual"),
		Confirm:            stripe.Bool(true),
		PaymentMethodTypes: []*string{stripe.String("card")},
	}

	s.log.LogPayment("STRIPE", req.PaymentID, "Creating payment intent")
	pi, err := s.client.PaymentIntents.New(piParams)
	if err != nil {
		s.log.Error("STRIPE", fmt.Sprintf("Failed to create payment intent: %v", err))
		return nil, fmt.Errorf("%w: %v", ErrStripeAPIError, err)
	}
	s.log.LogPayment("STRIPE", req.PaymentID, fmt.Sprintf("Payment intent created: %s", pi.ID))

	// Handle payment intent status
	var status models.PaymentStatus
	switch pi.Status {
	case stripe.PaymentIntentStatusSucceeded:
		status = models.StatusSuccess
		s.log.LogPayment("STRIPE", req.PaymentID, "Payment succeeded")
	case stripe.PaymentIntentStatusProcessing:
		status = models.StatusPending
		s.log.LogPayment("STRIPE", req.PaymentID, "Payment is processing")
	case stripe.PaymentIntentStatusRequiresAction:
		status = models.StatusPending
		s.log.LogPayment("STRIPE", req.PaymentID, "Payment requires further action")
	default:
		status = models.StatusFailed
		s.log.LogPayment("STRIPE", req.PaymentID, fmt.Sprintf("Payment failed with status: %s", pi.Status))
	}

	// Create response
	response := &models.StripePaymentResponse{
		PaymentID:     req.PaymentID,
		OrderID:       req.OrderID,
		Status:        status,
		Amount:        float64(pi.Amount) / 100.0, // Convert back from cents
		Currency:      string(pi.Currency),
		TransactionID: pi.ID,
		PaymentMethod: paymentMethod,
		Created:       pi.Created,
	}

	if pi.LatestCharge != nil && pi.LatestCharge.ID != "" {
		charge, err := s.client.Charges.Get(pi.LatestCharge.ID, nil)
		if err == nil && charge.ReceiptURL != "" {
			response.ReceiptURL = charge.ReceiptURL
		}
	}

	return response, nil
}

// We need to add this method to fetch payment by order ID
func (s *StripeService) getPaymentByOrderID(orderID string) (*models.Payment, error) {
	// This is a mock implementation - in a real app, you would query the database
	// In this case, we're returning a placeholder payment
	// This function should be replaced with actual database access

	s.log.LogPayment("LOOKUP", orderID, "Looking up payment by order ID (mock implementation)")

	// Return a mock payment with the given order ID
	return &models.Payment{
		PaymentID:     fmt.Sprintf("pay_%s", orderID),
		OrderID:       orderID,
		Status:        models.StatusSuccess,
		Price:         99.99,                         // Mock price
		TransactionID: fmt.Sprintf("pi_%s", orderID), // Mock Stripe payment intent ID
		CreatedDate:   time.Now().Add(-24 * time.Hour),
		UpdatedDate:   time.Now().Add(-24 * time.Hour),
	}, nil
}

// RefundPayment refunds a payment through Stripe
func (s *StripeService) RefundPayment(ctx context.Context, req *models.StripeRefundRequest) (*models.Payment, error) {
	logIdentifier := req.OrderID // Use OrderID for logging
	s.log.LogPayment("REFUND", logIdentifier, "Processing Stripe refund")

	// Fetch the payment by order ID to get transaction details
	payment, err := s.getPaymentByOrderID(req.OrderID)
	if err != nil {
		s.log.Error("STRIPE", fmt.Sprintf("Failed to fetch payment for order %s: %v", req.OrderID, err))
		return nil, fmt.Errorf("failed to fetch payment: %w", err)
	}

	// Get the transaction ID (Stripe payment intent ID) from the payment record
	paymentIntentID := payment.TransactionID
	if paymentIntentID == "" {
		s.log.Error("STRIPE", fmt.Sprintf("No transaction ID for payment with order ID %s", req.OrderID))
		return nil, fmt.Errorf("payment has no transaction ID")
	}

	// Create refund parameters
	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(paymentIntentID),
		Reason:        stripe.String(string(stripe.RefundReasonRequestedByCustomer)),
	}

	// We'll always refund the full amount as it's fetched from the database
	s.log.LogPayment("REFUND", req.OrderID, "Refunding full amount")

	// Process the refund
	refundObj, err := s.client.Refunds.New(params)
	if err != nil {
		s.log.Error("STRIPE", fmt.Sprintf("Refund failed: %v", err))
		return nil, fmt.Errorf("%w: %v", ErrStripeAPIError, err)
	}

	s.log.LogPayment("REFUND", req.OrderID, fmt.Sprintf("Refund successful, refund ID: %s", refundObj.ID))

	// Update the payment record with refund details
	refundedPayment := payment // Use the payment we fetched earlier
	refundedPayment.Status = models.StatusRefunded
	refundedPayment.UpdatedDate = time.Now()

	// Set the refund reference URL for tracking
	refundedPayment.URL = fmt.Sprintf("https://payment.gateway.com/stripe/refunds/%s", refundObj.ID)

	// Note: The calling handler is responsible for updating the database and publishing events
	return refundedPayment, nil
}

// GetPaymentDetails retrieves payment details from Stripe
func (s *StripeService) GetPaymentDetails(ctx context.Context, paymentIntentID string) (*models.StripePaymentResponse, error) {
	s.log.LogPayment("GET", paymentIntentID, "Retrieving payment details from Stripe")

	pi, err := s.client.PaymentIntents.Get(paymentIntentID, nil)
	if err != nil {
		s.log.Error("STRIPE", fmt.Sprintf("Failed to retrieve payment intent: %v", err))
		return nil, fmt.Errorf("%w: %v", ErrStripeAPIError, err)
	}

	// Map Stripe status to our status
	var status models.PaymentStatus
	switch pi.Status {
	case stripe.PaymentIntentStatusSucceeded:
		status = models.StatusSuccess
	case stripe.PaymentIntentStatusProcessing:
		status = models.StatusPending
	case stripe.PaymentIntentStatusCanceled:
		status = models.StatusCancelled
	default:
		status = models.StatusFailed
	}

	// Extract order ID from metadata
	orderID := ""
	if val, ok := pi.Metadata["order_id"]; ok {
		orderID = val
	}

	// Extract payment ID from metadata
	paymentID := ""
	if val, ok := pi.Metadata["payment_id"]; ok {
		paymentID = val
	} else {
		paymentID = paymentIntentID // Use Stripe ID as fallback
	}

	response := &models.StripePaymentResponse{
		PaymentID:     paymentID,
		OrderID:       orderID,
		Status:        status,
		Amount:        float64(pi.Amount) / 100.0,
		Currency:      string(pi.Currency),
		TransactionID: pi.ID,
		PaymentMethod: pi.PaymentMethod.ID,
		Created:       pi.Created,
	}

	if pi.LatestCharge != nil && pi.LatestCharge.ID != "" {
		charge, err := s.client.Charges.Get(pi.LatestCharge.ID, nil)
		if err == nil && charge.ReceiptURL != "" {
			response.ReceiptURL = charge.ReceiptURL
		}
	}

	return response, nil
}
