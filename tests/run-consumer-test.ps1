#!/usr/bin/env pwsh
# Script to test the Kafka consumer functionality
# Usage: .\tests\run-consumer-test.ps1

param(
    [switch]$skipIntegrationTests = $false,
    [switch]$skipMigration = $true  # Changed default to true to skip migrations by default
)

Write-Host "===== Testing Kafka Consumer Functionality ====="

# Step 1: Run unit tests for Kafka consumer
Write-Host "`n[1/4] Running unit tests for Kafka consumer..."
if ($skipIntegrationTests) {
    go test -v ./tests/kafka -short
} else {
    # Debug mode: include a longer timeout for integration tests
    $env:KAFKA_TEST_TIMEOUT = "30s"
    go test -v ./tests/kafka
}

if ($LASTEXITCODE -ne 0) {
    Write-Host "Unit tests failed. Exiting." -ForegroundColor Red
    exit 1
}

# Step 2: Make sure Kafka and MySQL are running
Write-Host "`n[2/4] Checking if Kafka and MySQL are running..."
$runningContainers = docker ps --format "{{.Names}}"
$kafkaRunning = $runningContainers -like "*kafka*"
$mysqlRunning = $runningContainers -like "*mysql*"

if (-not $kafkaRunning) {
    Write-Host "Kafka container is not running. Starting docker-compose..." -ForegroundColor Yellow
    docker-compose up -d kafka zookeeper
    Write-Host "Waiting 10 seconds for Kafka to start..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10
}

if (-not $mysqlRunning) {
    Write-Host "MySQL container is not running. Starting docker-compose..." -ForegroundColor Yellow
    docker-compose up -d mysql
    Write-Host "Waiting 10 seconds for MySQL to start..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10
}

# Step 3: Run database migrations
Write-Host "`n[3/4] Running database migrations..."
if ($skipMigration) {
    Write-Host "Skipping database migration as requested..." -ForegroundColor Yellow
} else {
    go run ./cmd/migrate/main.go

    if ($LASTEXITCODE -ne 0) {
        Write-Host "Database migration failed. Exiting." -ForegroundColor Red
        exit 1
    }
}

# Step 4: Publish a test order and check if consumer processes it
Write-Host "`n[4/4] Publishing a test order to Kafka..."

# Start the application in the background
Write-Host "Starting the application..." -ForegroundColor Yellow
$tempLogFile = "app_startup.log"

# Create a log file first
"" | Out-File -FilePath $tempLogFile

# Start the process without redirecting output (this was causing the error)
$appProcess = Start-Process -FilePath "go" -ArgumentList "run main.go" -NoNewWindow -PassThru

try {
    # Wait longer for the application to start and consumer to connect
    Write-Host "Waiting 10 seconds for application to start..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10
    
    # Check if the application is running
    if ($appProcess -ne $null -and -not $appProcess.HasExited) {
        Write-Host "Application is running with process ID: $($appProcess.Id)" -ForegroundColor Green
    } else {
        Write-Host "Application may have failed to start properly." -ForegroundColor Red
        if ($appProcess -ne $null) {
            Write-Host "Exit code: $($appProcess.ExitCode)" -ForegroundColor Red
        }
    }

    # Publish a test order
    Write-Host "Publishing test order..." -ForegroundColor Yellow
    .\scripts\publish-test-order.ps1

    # Wait a moment for the consumer to process the message
    Write-Host "Waiting 10 seconds for consumer to process the message..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10

    # Query the database to see if a payment was created
    Write-Host "Checking if payment was created in database..." -ForegroundColor Yellow
    
    # Using the migrate tool to check the database
    $query = "SELECT payment_id, order_id, status, price, created_date FROM payments WHERE created_date > DATE_SUB(NOW(), INTERVAL 1 MINUTE)"
    $result = docker exec -it $(docker ps -q --filter "name=mysql") mysql -u payment_user -ppayment_pass payment_gateway -e "$query"
    
    if ($result -match "payment_id") {
        Write-Host "Success! Found recent payment records in database:" -ForegroundColor Green
        Write-Host $result -ForegroundColor Cyan
    } else {
        Write-Host "No recent payment records found in database." -ForegroundColor Red
        Write-Host "The consumer might not be working correctly." -ForegroundColor Red
    }
} finally {
    # Stop the application
    if ($appProcess -ne $null -and -not $appProcess.HasExited) {
        Write-Host "Stopping application..." -ForegroundColor Yellow
        try {
            Stop-Process -Id $appProcess.Id -Force
        } catch {
            Write-Host "Error stopping the application: $_" -ForegroundColor Red
        }
    }
    
    # Clean up log file
    if (Test-Path $tempLogFile) {
        Remove-Item $tempLogFile
    }
}

Write-Host "`n===== Kafka Consumer Test Complete ====="
