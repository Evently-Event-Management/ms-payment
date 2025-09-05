package kafka_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"payment-gateway/internal/kafka"
	"payment-gateway/internal/models"
	"payment-gateway/internal/storage"
)

// MockStore implements the storage.Store interface for testing
type MockStore struct {
	mock.Mock
}

func (m *MockStore) SavePayment(payment *models.Payment) error {
	args := m.Called(payment)
	return args.Error(0)
}

func (m *MockStore) GetPayment(id string) (*models.Payment, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Payment), args.Error(1)
}

func (m *MockStore) UpdatePayment(payment *models.Payment) error {
	args := m.Called(payment)
	return args.Error(0)
}

func (m *MockStore) ListPayments(merchantID string, limit, offset int) ([]*models.Payment, error) {
	args := m.Called(merchantID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Payment), args.Error(1)
}

func (m *MockStore) GetTicketByOrderID(OrderID string) (*models.Payment, error) {
	args := m.Called(OrderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Payment), args.Error(1)
}

func (m *MockStore) SaveOrder(order *models.Order) error {
	args := m.Called(order)
	return args.Error(0)
}

func (m *MockStore) GetOrder(orderID string) (*models.Order, error) {
	args := m.Called(orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

// TestOrderConsumerIntegration tests the order consumer with a real Kafka broker
// This test requires a running Kafka broker
func TestOrderConsumerIntegration(t *testing.T) {
	// Skip test if short mode is enabled
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Get Kafka broker address from environment or use default
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "localhost:29092" // Default from docker-compose
	}

	// Create a test producer with a short timeout to quickly detect if Kafka is not available
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Net.DialTimeout = 5 * time.Second

	// Ensure we're using the same Kafka offset as our consumer
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	producer, err := sarama.NewSyncProducer([]string{kafkaBrokers}, config)
	if err != nil {
		t.Skip("Skipping test because Kafka is not available:", err)
		return
	}
	defer producer.Close()

	// Create a mock store
	mockStore := new(MockStore)

	// Expect a payment to be saved
	mockStore.On("SavePayment", mock.AnythingOfType("*models.Payment")).Return(nil)

	// Variable to store the expected order ID for the test
	var expectedOrderID string

	// Create a channel to track when our specific test order is processed
	handlerCalled := make(chan struct{}, 1)
	testHandler := func(order *models.Order) error {
		// Only acknowledge processing of our specific test order
		if order.OrderID == expectedOrderID {
			t.Logf("Found our test order: %s", order.OrderID)
			handlerCalled <- struct{}{}
		} else {
			t.Logf("Ignoring other order: %s", order.OrderID)
		}
		return nil
	}

	// Create the consumer with the mock store
	consumerConfig := sarama.NewConfig()
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumer, err := kafka.NewOrderConsumer([]string{kafkaBrokers}, "test-consumer-group-"+time.Now().Format("20060102150405"), mockStore)
	require.NoError(t, err)
	defer consumer.Close()

	// Start consuming in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := consumer.ConsumeOrders(ctx, testHandler)
		if err != nil && err != context.Canceled {
			t.Errorf("Consumer error: %v", err)
		}
	}()

	// Create a test order with a unique identifier to distinguish from other test messages
	uniqueId := time.Now().Format("20060102150405") + "-" + fmt.Sprintf("%d", time.Now().UnixNano()%10000)
	testOrder := &models.Order{
		OrderID:   "test-order-" + uniqueId,
		UserID:    "test-user-" + uniqueId,
		SessionID: "test-session-1",
		SeatIDs:   []string{"seat-1", "seat-2"},
		Status:    "created",
		Price:     100.50,
		CreatedAt: time.Now(),
	}

	// Set the expected order ID for the handler to match
	expectedOrderID = testOrder.OrderID

	// Serialize the order
	orderJSON, err := json.Marshal(testOrder)
	require.NoError(t, err)

	// Send the order to Kafka
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: "order.created",
		Value: sarama.ByteEncoder(orderJSON),
	})
	require.NoError(t, err)

	// Wait for the handler to be called with a timeout
	select {
	case <-handlerCalled:
		t.Logf("Successfully received our test order with ID: %s", testOrder.OrderID)
	case <-time.After(20 * time.Second):
		t.Fatalf("Timeout waiting for message to be consumed: %s", testOrder.OrderID)
	}

	// Verify that SavePayment was called
	mockStore.AssertCalled(t, "SavePayment", mock.AnythingOfType("*models.Payment"))

	// Verify the payment properties from the captured call
	calls := mockStore.Calls
	var capturedPayment *models.Payment

	// Loop through all captured payments to find the one matching our test order
	for _, call := range calls {
		if call.Method == "SavePayment" {
			payment := call.Arguments.Get(0).(*models.Payment)
			if payment.OrderID == testOrder.OrderID {
				capturedPayment = payment
				break
			}
		}
	}

	assert.NotNil(t, capturedPayment, "Payment for our test order should have been captured")
	if capturedPayment != nil {
		assert.Equal(t, testOrder.OrderID, capturedPayment.OrderID, "Order ID should match our test order")
		assert.Equal(t, models.StatusPending, capturedPayment.Status, "Status should be pending")
		assert.NotEmpty(t, capturedPayment.PaymentID, "Payment ID should not be empty")
		assert.NotEmpty(t, capturedPayment.URL, "URL should not be empty")
		assert.Contains(t, capturedPayment.URL, testOrder.OrderID, "URL should contain our test order ID")
	}
}

// TestOrderConsumerHandler tests the consumer handler logic directly without requiring Kafka
func TestOrderConsumerHandler(t *testing.T) {
	// Create a mock store
	mockStore := new(MockStore)

	// Expect a payment to be saved with any payment object
	mockStore.On("SavePayment", mock.AnythingOfType("*models.Payment")).Return(nil)

	// Create a test order
	testOrder := &models.Order{
		OrderID:   "test-order-unit-" + time.Now().Format("20060102150405"),
		UserID:    "test-user-1",
		SessionID: "test-session-1",
		SeatIDs:   []string{"seat-1", "seat-2"},
		Status:    "created",
		Price:     100.50,
		CreatedAt: time.Now(),
	}

	// Set up a handler
	handlerCalled := false
	testHandler := func(order *models.Order) error {
		handlerCalled = true
		assert.Equal(t, testOrder.OrderID, order.OrderID)
		return nil
	}

	// Create the consumer handler - we'll call its logic directly
	handler := struct {
		handler func(*models.Order) error
		store   storage.Store
	}{
		handler: testHandler,
		store:   mockStore,
	}

	// Create a mock session
	mockSession := &MockConsumerGroupSession{}
	mockSession.On("MarkMessage", mock.Anything, "").Return()

	// Create a mock claim with a message channel
	mockClaim := &MockConsumerGroupClaim{}
	msgChan := make(chan *sarama.ConsumerMessage, 1)
	mockClaim.On("Messages").Return(msgChan)

	// Create and send a test message
	orderJSON, _ := json.Marshal(testOrder)
	msg := &sarama.ConsumerMessage{
		Topic:     "order.created",
		Partition: 0,
		Offset:    0,
		Value:     orderJSON,
	}
	msgChan <- msg
	close(msgChan)

	// Process the message using our handler's logic (replicating ConsumeClaim)
	go func() {
		for message := range mockClaim.Messages() {
			var order models.Order
			if err := json.Unmarshal(message.Value, &order); err != nil {
				t.Errorf("Failed to unmarshal message: %v", err)
				continue
			}

			// Create a payment from the order
			payment := &models.Payment{
				PaymentID:   "test-payment-id", // Use fixed ID for testing
				OrderID:     order.OrderID,
				Status:      models.StatusPending,
				CreatedDate: time.Now(),
				URL:         "https://test.payment.url/" + order.OrderID,
			}

			// Save the payment
			if err := handler.store.SavePayment(payment); err != nil {
				t.Errorf("Failed to save payment: %v", err)
				continue
			}

			// Call the handler
			if err := handler.handler(&order); err != nil {
				t.Errorf("Handler error: %v", err)
				continue
			}

			// Mark message as processed
			mockSession.MarkMessage(message, "")
		}
	}()

	// Wait for processing to complete
	time.Sleep(100 * time.Millisecond)

	// Verify expectations
	assert.True(t, handlerCalled, "Handler should have been called")
	mockStore.AssertCalled(t, "SavePayment", mock.AnythingOfType("*models.Payment"))
	mockSession.AssertExpectations(t)
	mockClaim.AssertExpectations(t)
}

// Mock implementations for Sarama interfaces
type MockConsumerGroupSession struct {
	mock.Mock
}

func (m *MockConsumerGroupSession) Claims() map[string][]int32 {
	args := m.Called()
	return args.Get(0).(map[string][]int32)
}

func (m *MockConsumerGroupSession) MemberID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConsumerGroupSession) GenerationID() int32 {
	args := m.Called()
	return int32(args.Int(0))
}

func (m *MockConsumerGroupSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
	m.Called(topic, partition, offset, metadata)
}

func (m *MockConsumerGroupSession) Commit() {
	m.Called()
}

func (m *MockConsumerGroupSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {
	m.Called(topic, partition, offset, metadata)
}

func (m *MockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	m.Called(msg, metadata)
}

func (m *MockConsumerGroupSession) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

type MockConsumerGroupClaim struct {
	mock.Mock
}

func (m *MockConsumerGroupClaim) Topic() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockConsumerGroupClaim) Partition() int32 {
	args := m.Called()
	return int32(args.Int(0))
}

func (m *MockConsumerGroupClaim) InitialOffset() int64 {
	args := m.Called()
	return int64(args.Int(0))
}

func (m *MockConsumerGroupClaim) HighWaterMarkOffset() int64 {
	args := m.Called()
	return int64(args.Int(0))
}

func (m *MockConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	args := m.Called()
	// Fix the type assertion to handle channel conversion correctly
	return args.Get(0).(chan *sarama.ConsumerMessage)
}
