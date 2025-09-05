package kafka

import (
	"payment-gateway/internal/models"
	"payment-gateway/internal/storage"

	"github.com/IBM/sarama"
)

// OrderConsumerHandler is exported for testing purposes
type OrderConsumerHandler struct {
	Handler func(*models.Order) error
	Store   storage.Store
}

// ConsumeClaim processes Kafka messages
func (h *OrderConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	return (&orderConsumerHandler{
		handler: h.Handler,
		store:   h.Store,
	}).ConsumeClaim(session, claim)
}

// Setup is called before consuming starts
func (h *OrderConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is called after consuming ends
func (h *OrderConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}
