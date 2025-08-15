package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"
	"payment-gateway/internal/models"
)

type Consumer struct {
	consumer sarama.ConsumerGroup
	topics   []string
}

func NewConsumer(brokers []string, groupID string) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	topics := []string{"payment-events", "payment-webhooks", "payment-notifications"}

	return &Consumer{
		consumer: consumer,
		topics:   topics,
	}, nil
}

func (c *Consumer) ConsumePayments(ctx context.Context, handler func(*models.PaymentEvent) error) error {
	consumerHandler := &paymentConsumerHandler{handler: handler}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := c.consumer.Consume(ctx, c.topics, consumerHandler); err != nil {
				log.Printf("Error consuming messages: %v", err)
				return err
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.consumer.Close()
}

type paymentConsumerHandler struct {
	handler func(*models.PaymentEvent) error
}

func (h *paymentConsumerHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (h *paymentConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *paymentConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		var event models.PaymentEvent
		if err := json.Unmarshal(message.Value, &event); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}

		if err := h.handler(&event); err != nil {
			log.Printf("Failed to handle payment event: %v", err)
			continue
		}

		session.MarkMessage(message, "")
	}

	return nil
}
