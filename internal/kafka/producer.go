package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"payment-gateway/internal/models"
	"payment-gateway/internal/logger"
)

type Producer struct {
	producer sarama.SyncProducer
	mockMode bool
	log      *logger.Logger
}

func NewProducer(brokers []string, mockMode bool, log *logger.Logger) (*Producer, error) {
	if mockMode {
		log.LogKafka("MOCK_MODE", "producer", "Running in mock mode - no actual Kafka connection")
		return &Producer{
			producer: nil,
			mockMode: true,
			log:      log,
		}, nil
	}

	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}

	log.LogKafka("CONNECTED", "producer", fmt.Sprintf("Connected to Kafka brokers: %v", brokers))
	return &Producer{
		producer: producer,
		mockMode: false,
		log:      log,
	}, nil
}

func (p *Producer) PublishPaymentEvent(event *models.PaymentEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	topic := p.getTopicForEvent(event.Type)
	
	if p.mockMode {
		p.log.LogKafka("MOCK_PUBLISH", topic, fmt.Sprintf("Mock publishing event: %s for payment: %s", event.Type, event.PaymentID))
		p.log.LogKafka("MOCK_DATA", topic, string(data))
		return nil
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(event.PaymentID),
		Value: sarama.ByteEncoder(data),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.log.Error("KAFKA", fmt.Sprintf("Failed to send message to topic %s: %v", topic, err))
		return fmt.Errorf("failed to send message: %w", err)
	}

	p.log.LogKafka("PUBLISHED", topic, fmt.Sprintf("Message sent to partition %d at offset %d for payment %s", partition, offset, event.PaymentID))
	return nil
}

func (p *Producer) getTopicForEvent(eventType string) string {
	switch eventType {
	case "payment.success":
		return "payment-success"
	case "payment.failed":
		return "payment-failed"
	case "payment.refunded":
		return "payment-refunded"
	default:
		return "payment-events"
	}
}

func (p *Producer) Close() error {
	if p.mockMode {
		p.log.LogKafka("MOCK_CLOSE", "producer", "Mock producer closed")
		return nil
	}
	
	if p.producer != nil {
		p.log.LogKafka("CLOSING", "producer", "Closing Kafka producer connection")
		return p.producer.Close()
	}
	return nil
}
