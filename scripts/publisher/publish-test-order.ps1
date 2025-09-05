#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Publish a test order to Kafka.

.DESCRIPTION
    Generates a test order JSON payload and publishes it to the Kafka topic.
    Works with local kcat/kafkacat installation or falls back to Docker container.

.PARAMETER kafkaBroker
    Kafka broker to publish to (default: "kafka_pay:9092").

.PARAMETER topic
    Kafka topic to publish to (default: "order.created").

.PARAMETER orderId
    Optional order ID. If empty, a new UUID is generated.

.PARAMETER price
    Price of the order (default: 99.99).
#>

param (
    [string]$kafkaBroker = "kafka_pay:9092",
    [string]$topic = "order.created",
    [string]$orderId = "",
    [decimal]$price = 99.99
)

# Generate UUID for order if not provided
if ($orderId -eq "") { $orderId = [guid]::NewGuid().ToString() }

# Build JSON payload
$orderData = @{
    "orderID"   = $orderId
    "userID"    = [guid]::NewGuid().ToString()
    "sessionID" = [guid]::NewGuid().ToString()
    "seatIDs"   = @([guid]::NewGuid().ToString(), [guid]::NewGuid().ToString())
    "status"    = "created"
    "price"     = $price
    "createdAt" = (Get-Date).ToString("o")
}

$jsonPayload = $orderData | ConvertTo-Json -Compress

Write-Host "Publishing payload:" -ForegroundColor Cyan
Write-Host $jsonPayload

# Check for local kafkacat/kcat
$kafkacatExists = Get-Command kafkacat -ErrorAction SilentlyContinue
$kcatExists    = Get-Command kcat -ErrorAction SilentlyContinue

if ($kafkacatExists) {
    Write-Host "Using local kafkacat..." -ForegroundColor Yellow
    $jsonPayload | kafkacat -b $kafkaBroker -t $topic -P
}
elseif ($kcatExists) {
    Write-Host "Using local kcat..." -ForegroundColor Yellow
    $jsonPayload | kcat -b $kafkaBroker -t $topic -P
}
else {
    Write-Host "Local kafkacat/kcat not found. Using Docker container..." -ForegroundColor Yellow
    # Use Docker container and pipe JSON
    $jsonPayload | docker run --rm -i --network ms-payment_default confluentinc/cp-kafkacat `
        sh -c "kafkacat -b $kafkaBroker -t $topic -P"
}

Write-Host "✅ Published order with ID: $orderId to topic: $topic on broker: $kafkaBroker" -ForegroundColor Green
