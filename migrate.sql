-- Migration file for payment-gateway database
-- Version: 1.0
-- Date: September 5, 2025
-- Description: Initial schema migration for payments table

-- Set the collation for the session
SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;
-- drop table payments; -- Commented out to preserve existing data
-- Create migration tracking table if it doesn't exist
CREATE TABLE IF NOT EXISTS migrations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    version VARCHAR(50) NOT NULL COLLATE utf8mb4_unicode_ci,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    description TEXT COLLATE utf8mb4_unicode_ci,
    
    UNIQUE KEY uk_version (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Check if migration has already been applied
SET @migration_version = '1.0.0';
SET @migration_description = 'Initial payments table schema';

-- Only run this migration if it hasn't been applied yet
SET @migration_exists = (SELECT COUNT(*) FROM migrations WHERE version = @migration_version COLLATE utf8mb4_unicode_ci);
SET @sql = IF(@migration_exists = 0, 
    'DO 1', 
    'SELECT \'Migration already applied\' AS message');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- If migration doesn't exist, apply it
INSERT INTO migrations (version, description)
SELECT @migration_version, @migration_description
WHERE NOT EXISTS (SELECT 1 FROM migrations WHERE version = @migration_version COLLATE utf8mb4_unicode_ci);

-- Main migration: Create simplified payments table
CREATE TABLE IF NOT EXISTS payments (
    payment_id CHAR(36) PRIMARY KEY COMMENT 'UUID format',
    order_id CHAR(36) NOT NULL COMMENT 'UUID format',
    status VARCHAR(50) NOT NULL COLLATE utf8mb4_unicode_ci,
    price DECIMAL(10, 2) NOT NULL DEFAULT 0.00 COMMENT 'Payment amount',
    created_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    url VARCHAR(500) COLLATE utf8mb4_unicode_ci,
    
    INDEX idx_order_id (order_id),
    INDEX idx_status (status),
    INDEX idx_created_date (created_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Insert sample data for testing
INSERT INTO payments (payment_id, order_id, status, price, created_date, url)
SELECT 
    UUID(), UUID(), 'success', 99.99, NOW(), 'https://payment.gateway.com/receipt/sample-1'
WHERE NOT EXISTS (SELECT 1 FROM payments LIMIT 1);

-- Insert more sample data with different statuses and timestamps
INSERT INTO payments (payment_id, order_id, status, price, created_date, url)
VALUES
    (UUID(), UUID(), 'pending', 149.99, DATE_SUB(NOW(), INTERVAL 2 DAY), 'https://payment.gateway.com/receipt/sample-2'),
    (UUID(), UUID(), 'failed', 29.99, DATE_SUB(NOW(), INTERVAL 1 DAY), 'https://payment.gateway.com/receipt/sample-3'),
    (UUID(), UUID(), 'success', 199.99, DATE_SUB(NOW(), INTERVAL 12 HOUR), 'https://payment.gateway.com/receipt/sample-4'),
    (UUID(), UUID(), 'refunded', 59.99, DATE_SUB(NOW(), INTERVAL 6 HOUR), 'https://payment.gateway.com/receipt/sample-5'),
    (UUID(), UUID(), 'pending', 79.99, DATE_SUB(NOW(), INTERVAL 3 HOUR), 'https://payment.gateway.com/receipt/sample-6'),
    (UUID(), UUID(), 'success', 129.99, DATE_SUB(NOW(), INTERVAL 1 HOUR), 'https://payment.gateway.com/receipt/sample-7'),
    (UUID(), UUID(), 'success', 89.99, NOW(), 'https://payment.gateway.com/receipt/sample-8'),
    (UUID(), UUID(), 'pending', 39.99, NOW(), 'https://payment.gateway.com/receipt/sample-9'),
    (UUID(), UUID(), 'success', 69.99, NOW(), 'https://payment.gateway.com/receipt/sample-10');

-- Log completion
SELECT CONCAT('Migration ', @migration_version, ' completed successfully') AS message;
