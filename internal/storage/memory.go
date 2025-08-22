package storage

import (
	"errors"
	"sync"

	"payment-gateway/internal/models"
)

type InMemoryStore struct {
	payments map[string]*models.Payment
	mutex    sync.RWMutex
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		payments: make(map[string]*models.Payment),
	}
}

func (s *InMemoryStore) SavePayment(payment *models.Payment) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.payments[payment.PaymentID] = payment
	return nil
}

func (s *InMemoryStore) GetPayment(id string) (*models.Payment, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	payment, exists := s.payments[id]
	if !exists {
		return nil, errors.New("payment not found")
	}

	return payment, nil
}

func (s *InMemoryStore) UpdatePayment(payment *models.Payment) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.payments[payment.PaymentID]; !exists {
		return errors.New("payment not found")
	}

	s.payments[payment.PaymentID] = payment
	return nil
}

func (s *InMemoryStore) ListPayments(orderID string, limit, offset int) ([]*models.Payment, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var payments []*models.Payment
	count := 0

	for _, payment := range s.payments {
		if payment.OrderID == orderID {
			if count >= offset && len(payments) < limit {
				payments = append(payments, payment)
			}
			count++
		}
	}

	return payments, nil
}
