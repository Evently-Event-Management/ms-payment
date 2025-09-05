# Payment Gateway Service with Stripe Integration

A comprehensive payment gateway built in Go with Stripe integration, proper error handling, Kafka integration, and load handling capabilities.

## Features

- **RESTful API** for payment processing
- **Stripe Integration** for real payment processing
- **Credit Card Validation** for validating cards before processing
- **Kafka Integration** for event streaming
- **Proper Error Handling** with detailed error responses
- **Load Handling** with rate limiting and graceful shutdown
- **Payment Processing** with Stripe
- **Refund Support** with partial and full refund capabilities
- **Card Validation** with expiry and format checks
- **Transaction Tracking** with unique IDs
- **Concurrent Processing** with goroutines
- **Docker Support** with multi-stage builds
- **OTP Verification** for secure payments

## API Endpoints

### Standard Payment API
```
POST /api/v1/payments/process        # Process a regular payment
GET /api/v1/payments/{id}            # Get payment details
GET /api/v1/payments/{id}/status     # Get payment status
POST /api/v1/payments/{id}/refund    # Refund a payment
POST /api/v1/payments/OTP            # Generate OTP for payment verification
POST /api/v1/payments/validate       # Validate OTP
```

### Stripe API
```
POST /api/v1/stripe/validate-card    # Validate credit card
POST /api/v1/stripe/payment          # Process payment with Stripe
POST /api/v1/stripe/refund           # Refund a Stripe payment
GET /api/v1/stripe/payment/{id}      # Get Stripe payment details
POST /api/v1/stripe/webhook          # Handle Stripe webhook events
```

### Health Check
```
GET /health
```

## Quick Start

### Prerequisites
- Go 1.18 or later
- MySQL database
- Redis server
- Kafka (for event processing)
- Stripe account (for payment processing)

### Setup

1. Clone the repository:
   ```
   git clone https://github.com/Evently-Event-Management/ms-payment.git
   cd ms-payment
   ```

2. Set up environment variables:
   ```
   cp .env.example .env
   ```
   
3. Edit the `.env` file and add your configuration details, including your Stripe API key.

4. Install dependencies:
   ```
   go mod download
   ```

5. Run the application:
   ```
   go run main.go
   ```

### Using Docker Compose (Recommended)
```bash
# Start all services (Kafka, Zookeeper, Redis, Payment Gateway)
make docker-run

# Or manually
docker-compose up --build
```

### Local Development
```bash
# Start Kafka and dependencies
make kafka-up

# In another terminal, run the application
make run
```

## Example Usage

### Validate a Credit Card
```bash
curl -X POST http://localhost:8085/api/v1/stripe/validate-card \
  -H "Content-Type: application/json" \
  -d '{
    "card": {
      "number": "4242424242424242",
      "exp_month": "12",
      "exp_year": "2025",
      "cvc": "123"
    }
  }'
```

### Process a Payment with Stripe
```bash
curl -X POST http://localhost:8085/api/v1/stripe/payment \
  -H "Content-Type: application/json" \
  -d '{
    "payment_id": "pay_123456",
    "order_id": "order_123456",
    "amount": 99.99,
    "currency": "usd",
    "description": "Test payment",
    "card": {
      "number": "4242424242424242",
      "exp_month": "12",
      "exp_year": "2025",
      "cvc": "123",
      "name": "John Doe"
    }
  }'
```

### Check Payment Status
```bash
curl http://localhost:8085/api/v1/payments/{payment_id}/status
```

## Test Cards

For testing with Stripe, you can use these test cards:

- **Success**: `4242424242424242`
- **Requires Authentication**: `4000002500003155`
- **Declined**: `4000000000000002`

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
