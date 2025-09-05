package storage

import (
	"payment-gateway/internal/models"
)

type Store interface {
	SavePayment(payment *models.Payment) error
	GetPayment(id string) (*models.Payment, error)
	UpdatePayment(payment *models.Payment) error
	ListPayments(merchantID string, limit, offset int) ([]*models.Payment, error)
	GetTicketByOrderID(OrderID string) (*models.Payment, error)

	// Order related operations
	SaveOrder(order *models.Order) error
	GetOrder(orderID string) (*models.Order, error)
}
