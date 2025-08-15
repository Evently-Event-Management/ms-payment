# Payment Gateway Mock Service

A comprehensive mock payment gateway built in Go with proper error handling, Kafka integration, and load handling capabilities.

## Features

- **RESTful API** for payment processing
- **Kafka Integration** for event streaming
- **Proper Error Handling** with detailed error responses
- **Load Handling** with rate limiting and graceful shutdown
- **Mock Payment Processing** with realistic success/failure scenarios
- **Refund Support** with partial and full refund capabilities
- **Card Validation** with expiry and format checks
- **Transaction Tracking** with unique IDs
- **Concurrent Processing** with goroutines
- **Docker Support** with multi-stage builds

## API Endpoints

### Process Payment
\`\`\`
POST /api/v1/payments/process
\`\`\`

### Get Payment
\`\`\`
GET /api/v1/payments/{id}
\`\`\`

### Get Payment Status
\`\`\`
GET /api/v1/payments/{id}/status
\`\`\`

### Refund Payment
\`\`\`
POST /api/v1/payments/{id}/refund
\`\`\`

### Health Check
\`\`\`
GET /health
\`\`\`

## Quick Start

### Using Docker Compose (Recommended)
\`\`\`bash
# Start all services (Kafka, Zookeeper, Redis, Payment Gateway)
make docker-run

# Or manually
docker-compose up --build
\`\`\`

### Local Development
\`\`\`bash
# Start Kafka and dependencies
make kafka-up

# In another terminal, run the application
make run
\`\`\`

## Example Usage

### Process a Payment
\`\`\`bash
curl -X POST http://localhost:8080/api/v1/payments/process \
  -H "Content-Type: application/json" \
  -d '{
    "merchant_id": "merchant_123",
    "amount": 99.99,
    "currency": "USD",
    "payment_method": "card",
    "card_number": "4111111111111111",
    "card_holder_name": "John Doe",
    "expiry_month": 12,
    "expiry_year": 2025,
    "cvv": "123",
    "description": "Test payment"
  }'
\`\`\`

### Check Payment Status
\`\`\`bash
curl http://localhost:8080/api/v1/payments/{payment_id}/status
\`\`\`

## Test Cards

- **Success**: `4111111111111111`
- **Failure**: `4000000000000002`
- **Insufficient Funds**: `4000000000000010`

## Architecture

The application follows clean architecture principles:

- **Handlers**: HTTP request handling and validation
- **Services**: Business logic and payment processing
- **Storage**: Data persistence (in-memory for demo)
- **Kafka**: Event streaming and messaging
- **Middleware**: Cross-cutting concerns (logging, CORS, rate limiting)

## Kafka Topics

- `payment-events`: General payment events
- `payment-success`: Successful payments
- `payment-failed`: Failed payments
- `payment-refunded`: Refunded payments

## Configuration

The application uses environment variables for configuration:

- `KAFKA_BROKERS`: Kafka broker addresses (default: localhost:9092)
- `SERVER_PORT`: HTTP server port (default: 8080)

## Load Handling

- **Rate Limiting**: 100 requests per second per client
- **Graceful Shutdown**: 30-second timeout for ongoing requests
- **Concurrent Processing**: Async payment processing with goroutines
- **Connection Pooling**: Efficient resource utilization

## Error Handling

Comprehensive error handling with specific error types:

- `400 Bad Request`: Invalid input data
- `402 Payment Required`: Insufficient funds or declined
- `404 Not Found`: Payment not found
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: System errors

## Monitoring

- Health check endpoint at `/health`
- Structured logging with request/response details
- Kafka message tracking with partition/offset info

## Development

\`\`\`bash
# Format code
make fmt

# Run tests
make test

# Build binary
make build

# Clean artifacts
make clean
