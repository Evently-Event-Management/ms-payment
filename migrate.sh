#!/bin/bash
# Migration runner script for payment-gateway
# Usage: ./migrate.sh [environment]
# Example: ./migrate.sh dev

# Default to development environment if not specified
ENV=${1:-dev}
echo "Running migration for $ENV environment"

# Load environment variables from .env file
if [ -f ".env.$ENV" ]; then
    echo "Loading environment from .env.$ENV"
    source .env.$ENV
elif [ -f ".env" ]; then
    echo "Loading environment from .env"
    source .env
else
    echo "No .env file found, using default values"
fi

# Database connection details
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-3306}
DB_USER=${DB_USER:-root}
DB_PASS=${DB_PASS:-password}
DB_NAME=${DB_NAME:-payment_gateway}

echo "Connecting to MySQL at $DB_HOST:$DB_PORT as $DB_USER"

# Run migration
mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p$DB_PASS $DB_NAME < migrate.sql

# Check if migration was successful
if [ $? -eq 0 ]; then
    echo "Migration completed successfully"
else
    echo "Migration failed"
    exit 1
fi
