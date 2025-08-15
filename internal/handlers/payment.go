package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"payment-gateway/internal/models"
	"payment-gateway/internal/services"
	"payment-gateway/internal/utils"
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
		switch err {
		case services.ErrInsufficientFunds:
			c.JSON(http.StatusPaymentRequired, utils.ErrorResponse("Insufficient funds", err.Error()))
		case services.ErrInvalidCard:
			c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid card details", err.Error()))
		case services.ErrCardExpired:
			c.JSON(http.StatusBadRequest, utils.ErrorResponse("Card expired", err.Error()))
		case services.ErrPaymentDeclined:
			c.JSON(http.StatusPaymentRequired, utils.ErrorResponse("Payment declined", err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, utils.ErrorResponse("Payment processing failed", err.Error()))
		}
		return
	}

	response := &models.PaymentResponse{
		ID:            payment.ID,
		Status:        payment.Status,
		Amount:        payment.Amount,
		Currency:      payment.Currency,
		TransactionID: payment.TransactionID,
		Message:       "Payment processed successfully",
		CreatedAt:     payment.CreatedAt,
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
		"id":     payment.ID,
		"status": payment.Status,
	}

	c.JSON(http.StatusOK, utils.SuccessResponse("Payment status retrieved", statusResponse))
}

func (h *PaymentHandler) RefundPayment(c *gin.Context) {
	paymentID := c.Param("id")
	if paymentID == "" {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Payment ID is required", ""))
		return
	}

	var req models.RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid refund request", err.Error()))
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

	payment, err := h.paymentService.RefundPayment(c.Request.Context(), paymentID, refundAmount, req.Reason)
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

	c.JSON(http.StatusOK, utils.SuccessResponse("Refund processed successfully", payment))
}

func (h *PaymentHandler) validatePaymentRequest(req *models.PaymentRequest) error {
	// Add custom validation logic here
	// For example: validate card number format, expiry date, etc.
	return nil
}

func (h *PaymentHandler) OTP(c *gin.Context) {

	if err := c.ShouldBindJSON(&models.Req); err != nil {
		c.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid email", err.Error()))
		return
	}

	h.paymentService.OtpSender(models.Req.Email)
	c.JSON(http.StatusOK, utils.SuccessResponse("OTP sent Successfully.", "TTl= 5min"))
}
