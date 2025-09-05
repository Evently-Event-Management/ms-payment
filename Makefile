.PHONY: build run test clean docker-build docker-run dev dev-mock run-mock docker-check docker-fix mysql-setup mysql-init mysql-logs dev-with-db run-with-db fmt lint mocks install-dev help migrate

mysql-setup:
	@echo "🗄️  Setting up MySQL database..."
	@docker-compose up -d mysql
	@echo "⏳ Waiting for MySQL to be ready..."
	@timeout /t 10 > nul
	@docker-compose exec mysql mysql -uroot -prootpassword -e "CREATE DATABASE IF NOT EXISTS payment_gateway;"
	@echo "✅ MySQL database ready"

mysql-init:
	@echo "🗄️  Initializing MySQL with sample data..."
	@docker-compose exec mysql mysql -uroot -prootpassword payment_gateway < scripts/mysql-init.sql
	@echo "✅ MySQL initialized with sample data"

migrate:
	@echo "🗄️  Running database migrations..."
	@go run cmd/migrate/main.go -env dev
	@echo "✅ Database migrations completed"

mysql-logs:
	@echo "📋 MySQL logs:"
	@docker-compose logs mysql

# Build the application
build:
	go build -o bin/payment-gateway .

# Run the application
run:
	go run .

dev:
	@echo "🔥 Starting Payment Gateway in development mode with hot reload..."
	@go run dev-watch.go

dev-mock:
	@echo "🔥 Starting Payment Gateway in MOCK mode (no Docker required)..."
	@KAFKA_MOCK_MODE=true KAFKA_ENABLED=false go run dev-watch.go

run-mock:
	@echo "🚀 Starting Payment Gateway in MOCK mode..."
	@KAFKA_MOCK_MODE=true KAFKA_ENABLED=false go run .

docker-check:
	@echo "🐳 Checking Docker status..."
	@docker info > /dev/null 2>&1 && echo "✅ Docker is running" || echo "❌ Docker is not running or accessible"
	@docker-compose version > /dev/null 2>&1 && echo "✅ Docker Compose is available" || echo "❌ Docker Compose is not available"

docker-fix:
	@echo "🔧 Attempting to fix Docker issues..."
	@echo "1. Restarting Docker Desktop..."
	@taskkill /F /IM "Docker Desktop.exe" > nul 2>&1 || true
	@timeout /t 3 > nul
	@start "" "C:\Program Files\Docker\Docker\Docker Desktop.exe" || echo "Please start Docker Desktop manually"
	@echo "2. Waiting for Docker to start..."
	@timeout /t 10 > nul
	@make docker-check

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/ logs/

# Build Docker image
docker-build:
	docker build -t payment-gateway .

# Run with Docker Compose
docker-run:
	docker-compose up --build

# Run Kafka and dependencies only
kafka-up:
	docker-compose up zookeeper kafka redis

full-stack:
	@echo "🚀 Starting full stack (MySQL + Kafka + Redis + App)..."
	docker-compose up --build

# Stop all services
down:
	docker-compose down

# View logs
logs:
	docker-compose logs -f payment-gateway

app-logs:
	@echo "📋 Recent application logs:"
	@tail -f logs/payment-gateway-$(shell date +%Y-%m-%d).log

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Generate mocks (if using mockery)
mocks:
	mockery --all --output=mocks

install-dev:
	go install github.com/fsnotify/fsnotify@latest
	go install github.com/fatih/color@latest

help:
	@echo "Available commands:"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application normally"
	@echo "  run-mock     - Run in mock mode (no Docker/Kafka needed)"
	@echo "  dev          - Run with hot reload (recommended for development)"
	@echo "  dev-mock     - Run with hot reload in mock mode"
	@echo "  mysql-setup  - Setup MySQL database"
	@echo "  mysql-init   - Initialize MySQL with sample data"
	@echo "  full-stack   - Start complete stack (MySQL + Kafka + Redis + App)"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts and logs"
	@echo "  app-logs     - View application logs"
	@echo "  docker-check - Check Docker status"
	@echo "  docker-fix   - Attempt to fix Docker issues (Windows)"
	@echo "  docker-*     - Docker related commands"
	@echo "  kafka-up     - Start only Kafka dependencies"
	@echo "  dev-with-db  - Run Payment Gateway with MySQL database"
	@echo "  run-with-db  - Run Payment Gateway with MySQL database"
