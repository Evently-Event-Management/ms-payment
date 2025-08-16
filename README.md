# Payment Gateway

A Go-based payment gateway service supporting payment processing, OTP verification, refunds, and integration with MySQL, Redis, and Kafka.

## Features

- RESTful API for payment processing and status queries
- OTP (One-Time Password) verification for secure transactions
- Refund support
- Asynchronous payment processing with Kafka
- MySQL for persistent storage
- Redis for OTP and lock management
- Structured logging

## Tech Stack

- Go (Golang)
- Gin (HTTP API)
- MySQL (storage)
- Redis (OTP/locks)
- Kafka (event streaming)

## Getting Started

### Prerequisites

- Go 1.18+
- MySQL
- Redis
- Kafka

### Configuration

Set environment variables as needed (e.g., `REDIS_ADDR`).  
Edit `config.yaml` or use environment variables for DB, Kafka, and server settings.

### Installation

1. Clone the repository:
