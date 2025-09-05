package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"payment-gateway/internal/models"
	"payment-gateway/internal/storage"
	"payment-gateway/internal/utils"

	"github.com/IBM/sarama"
)

type OrderConsumer struct {
	consumer sarama.ConsumerGroup
	topics   []string
	store    storage.Store
}

func NewOrderConsumer(brokers []string, groupID string, store storage.Store) (*OrderConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create order consumer group: %w", err)
	}

	// Change topic to order.created
	topics := []string{"order.created"}

	return &OrderConsumer{
		consumer: consumer,
		topics:   topics,
		store:    store,
	}, nil
}

func (c *OrderConsumer) ConsumeOrders(ctx context.Context, handler func(*models.Order) error) error {
	consumerHandler := &orderConsumerHandler{
		handler: handler,
		store:   c.store,
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := c.consumer.Consume(ctx, c.topics, consumerHandler); err != nil {
				log.Printf("Error consuming order messages: %v", err)
				return err
			}
		}
	}
}

// ConsumePayments is an alias for ConsumeOrders to maintain compatibility with existing code
func (c *OrderConsumer) ConsumePayments(ctx context.Context, handler func(*models.Order) error) error {
	return c.ConsumeOrders(ctx, handler)
}

func (c *OrderConsumer) Close() error {
	return c.consumer.Close()
}

type orderConsumerHandler struct {
	handler func(*models.Order) error
	store   storage.Store
}

func (h *orderConsumerHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *orderConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *orderConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		log.Printf("Received message from topic %s, partition %d, offset %d",
			message.Topic, message.Partition, message.Offset)

		// Log the raw message before attempting to unmarshal
		rawMessage := string(message.Value)
		log.Printf("Raw message content: %s", rawMessage)

		// Check if the JSON is valid
		if !json.Valid(message.Value) {
			log.Printf("Invalid JSON in message: %s", rawMessage)
			session.MarkMessage(message, "")
			continue
		}

		var order models.Order
		if err := json.Unmarshal(message.Value, &order); err != nil {
			log.Printf("Failed to unmarshal order.created message: %v", err)
			session.MarkMessage(message, "")
			continue
		}

		log.Printf("Processing order: %s, status: %s", order.OrderID, order.Status)

		// Create a new payment record for this order
		payment := &models.Payment{
			PaymentID:   utils.GenerateUUID(),
			OrderID:     order.OrderID,
			Status:      models.StatusPending, // Initial status is pending
			Price:       order.Price,          // Use the price from the order
			CreatedDate: time.Now(),
			URL:         fmt.Sprintf("https://payment.gateway.com/checkout/%s", order.OrderID),
		}

		// Save the payment to database
		if err := h.store.SavePayment(payment); err != nil {
			log.Printf("Failed to save payment to database: %v", err)
			continue
		}

		log.Printf("Created payment record with ID %s for order %s", payment.PaymentID, order.OrderID)

		// Call the handler for any additional processing
		if err := h.handler(&order); err != nil {
			log.Printf("Failed to handle order.created event: %v", err)
			// Continue processing even if handler fails
			// Payment has already been created
		}

		// Mark message as processed
		session.MarkMessage(message, "")
		log.Printf("Successfully processed message and created payment for order %s", order.OrderID)
	}

	return nil
}
