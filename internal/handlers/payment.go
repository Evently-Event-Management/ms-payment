package handlers

import (
	"net/http"
	"strconv"

	"payment-gateway/internal/models"
	"payment-gateway/internal/services"
	"payment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	paymentService *services.PaymentService
}

func NewPaymentHandler(paymentService *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
	}
}

func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	var req models.PaymentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request payload", err.Error()))
		return
	}

	// Validate payment request
	if err := h.validatePaymentRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Validation failed", err.Error()))
		return
	}

	payment, err := h.paymentService.ProcessPayment(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Payment processing failed", err.Error()))
		return
	}

	response := &models.Payment{
		PaymentID:   payment.PaymentID,
		OrderID:     payment.OrderID,
		Status:      payment.Status,
		CreatedDate: payment.CreatedDate,
		URL:         payment.URL,
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment processed", response))
}

func (h *PaymentHandler) GetPayment(c *gin.Context) {
	paymentID := c.Param("id")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Payment ID is required", ""))
		return
	}

	payment, err := h.paymentService.GetPayment(c.Request.Context(), paymentID)
	if err != nil {
		if err == services.ErrPaymentNotFound {
			c.JSON(http.StatusNotFound, utils.ErrorResponse("Payment not found", err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to retrieve payment", err.Error()))
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment retrieved", payment))
}

func (h *PaymentHandler) GetPaymentStatus(c *gin.Context) {
	paymentID := c.Param("id")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Payment ID is required", ""))
		return
	}

	payment, err := h.paymentService.GetPayment(c.Request.Context(), paymentID)
	if err != nil {
		if err == services.ErrPaymentNotFound {
			c.JSON(http.StatusNotFound, utils.ErrorResponse("Payment not found", err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to retrieve payment status", err.Error()))
		return
	}

	statusResponse := gin.H{
		"payment_id": payment.PaymentID,
		"status":     payment.Status,
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment status retrieved", statusResponse))
}

func (h *PaymentHandler) RefundPayment(c *gin.Context) {
	var req models.RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid refund request", err.Error()))
		return
	}

	// Get orderID from request body
	orderID := req.OrderID
	if orderID == "" {
		// If orderID is not in the body, try to get it from URL params
		orderID = c.Param("order_id")
	}

	if orderID == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Order ID is required", ""))
		return
	}

	// Fetch payment by order ID
	payment, err := h.paymentService.GetPaymentByOrderID(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.ErrorResponse("Payment not found for this order", err.Error()))
		return
	}

	var refundAmount *float64
	if req.Amount != "" {
		amount, err := strconv.ParseFloat(req.Amount, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid refund amount", err.Error()))
			return
		}
		refundAmount = &amount
	}

	// Use the payment ID from the fetched payment
	refundedPayment, err := h.paymentService.RefundPayment(c.Request.Context(), payment.PaymentID, refundAmount, req.Reason)
	if err != nil {
		switch err {
		case services.ErrPaymentNotFound:
			c.JSON(http.StatusNotFound, utils.ErrorResponse("Payment not found", err.Error()))
		case services.ErrInvalidRefundAmount:
			c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid refund amount", err.Error()))
		case services.ErrPaymentNotRefundable:
			c.JSON(http.StatusBadRequest, utils.ErrorResponse("Payment cannot be refunded", err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Refund processing failed", err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Refund processed successfully", refundedPayment))
}

func (h *PaymentHandler) validatePaymentRequest(req *models.PaymentRequest) error {
	// Add any custom validation logic here if needed
	return nil
}
