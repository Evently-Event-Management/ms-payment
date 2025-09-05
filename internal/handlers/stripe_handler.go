package handlers

import (
	"net/http"

	"payment-gateway/internal/models"
	"payment-gateway/internal/services"
	"payment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
)

type StripeHandler struct {
	stripeService  *services.StripeService
	paymentService *services.PaymentService
}

func NewStripeHandler(stripeService *services.StripeService, paymentService *services.PaymentService) *StripeHandler {
	return &StripeHandler{
		stripeService:  stripeService,
		paymentService: paymentService,
	}
}

// ValidateCard validates credit card details without creating a charge
func (h *StripeHandler) ValidateCard(c *gin.Context) {
	var req models.StripeCardValidationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", err.Error()))
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

	// Require either token or card
	if req.Token == "" && req.Card == nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", "Either token or card must be provided"))
		return
	}

	result, err := h.stripeService.ProcessPayment(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Payment processing failed", err.Error()))
		return
	}

	if result.Status == models.StatusSuccess || result.Status == models.StatusPending {
		paymentReq := &models.PaymentRequest{
			PaymentID: result.PaymentID,
			OrderID:   result.OrderID,
			Status:    result.Status,
			Price:     result.Amount,
			Date:      utils.UnixTimeToTime(result.Created),
		}
		_, err := h.paymentService.ProcessPayment(c.Request.Context(), paymentReq)
		if err != nil {
			// Log the error but continue since the Stripe payment was successful
		}
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

	// Process refund using Stripe
	result, err := h.stripeService.RefundPayment(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Refund processing failed", err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Refund processed", result))
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
