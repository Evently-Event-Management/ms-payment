package storage

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"payment-gateway/internal/config"
	"payment-gateway/internal/logger"
	"payment-gateway/internal/models"
)

type MySQLStore struct {
	db  *sql.DB
	log *logger.Logger
}

func NewMySQLStore(cfg config.DatabaseConfig, log *logger.Logger) (*MySQLStore, error) {
	log.LogDatabase("CONNECT", "mysql", fmt.Sprintf("Connecting to MySQL at %s:%s", cfg.Host, cfg.Port))

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Error("DATABASE", "Failed to open MySQL connection: "+err.Error())
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.MaxLifetime)

	// Test connection
	if err := db.Ping(); err != nil {
		log.Error("DATABASE", "Failed to ping MySQL: "+err.Error())
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &MySQLStore{
		db:  db,
		log: log,
	}

	// Initialize tables
	if err := store.initTables(); err != nil {
		log.Error("DATABASE", "Failed to initialize tables: "+err.Error())
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	log.LogDatabase("SUCCESS", "mysql", "MySQL connection established and tables initialized")
	return store, nil
}

func (s *MySQLStore) initTables() error {
	s.log.LogDatabase("MIGRATE", "mysql", "Creating payments table if not exists")

	query := `
	CREATE TABLE IF NOT EXISTS payments (
		id VARCHAR(36) PRIMARY KEY,
		merchant_id VARCHAR(255) NOT NULL,
		amount DECIMAL(10,2) NOT NULL,
		currency VARCHAR(3) NOT NULL,
		status VARCHAR(50) NOT NULL,
		card_number VARCHAR(255) NOT NULL,
		cardholder_name VARCHAR(255) NOT NULL,
		expiry_month INT NOT NULL,
		expiry_year INT NOT NULL,
		cvv VARCHAR(4) NOT NULL,
		transaction_id VARCHAR(255),
		failure_reason TEXT,
		refunded_amount DECIMAL(10,2) DEFAULT 0.00,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		processed_at TIMESTAMP NULL,
		
		INDEX idx_merchant_id (merchant_id),
		INDEX idx_status (status),
		INDEX idx_created_at (created_at),
		INDEX idx_transaction_id (transaction_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
	`

	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create payments table: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", "Payments table ready")
	return nil
}

func (s *MySQLStore) SavePayment(payment *models.Payment) error {
	s.log.LogDatabase("INSERT", "mysql", fmt.Sprintf("Saving payment %s", payment.ID))

	query := `
	INSERT INTO payments (
		id, merchant_id, amount, currency, status, card_number, cardholder_name,
		expiry_month, expiry_year, cvv, transaction_id, created_at, updated_at, processed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		payment.ID, payment.MerchantID, payment.Amount, payment.Currency,
		payment.Status, payment.CardNumber, payment.CardHolderName,
		payment.ExpiryMonth, payment.ExpiryYear, payment.CVV,
		payment.TransactionID, payment.CreatedAt, payment.UpdatedAt, payment.ProcessedAt,
	)

	if err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Failed to save payment %s: %s", payment.ID, err.Error()))
		return fmt.Errorf("failed to save payment: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Payment %s saved successfully", payment.ID))
	return nil
}

func (s *MySQLStore) GetPayment(id string) (*models.Payment, error) {
	s.log.LogDatabase("SELECT", "mysql", fmt.Sprintf("Fetching payment %s", id))

	query := `
	SELECT id, merchant_id, amount, currency, status, card_number, cardholder_name,
		   expiry_month, expiry_year, cvv, transaction_id,
		   created_at, updated_at, processed_at
	FROM payments WHERE id = ?
	`

	payment := &models.Payment{}
	var processedAt sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&payment.ID, &payment.MerchantID, &payment.Amount, &payment.Currency,
		&payment.Status, &payment.CardNumber, &payment.CardHolderName,
		&payment.ExpiryMonth, &payment.ExpiryYear, &payment.CVV,
		&payment.TransactionID, &payment.CreatedAt, &payment.UpdatedAt, &processedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			s.log.LogDatabase("NOT_FOUND", "mysql", fmt.Sprintf("Payment %s not found", id))
			return nil, fmt.Errorf("payment not found")
		}
		s.log.Error("DATABASE", fmt.Sprintf("Failed to get payment %s: %s", id, err.Error()))
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}

	if processedAt.Valid {
		payment.ProcessedAt = &processedAt.Time
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Payment %s fetched successfully", id))
	return payment, nil
}

func (s *MySQLStore) UpdatePayment(payment *models.Payment) error {
	s.log.LogDatabase("UPDATE", "mysql", fmt.Sprintf("Updating payment %s", payment.ID))

	query := `
	UPDATE payments SET
		status = ?, transaction_id = ?, updated_at = ?, processed_at = ?
	WHERE id = ?
	`

	_, err := s.db.Exec(query,
		payment.Status, payment.TransactionID,
		payment.UpdatedAt, payment.ProcessedAt, payment.ID,
	)

	if err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Failed to update payment %s: %s", payment.ID, err.Error()))
		return fmt.Errorf("failed to update payment: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Payment %s updated successfully", payment.ID))
	return nil
}

func (s *MySQLStore) ListPayments(merchantID string, limit, offset int) ([]*models.Payment, error) {
	s.log.LogDatabase("SELECT", "mysql", fmt.Sprintf("Listing payments for merchant %s (limit: %d, offset: %d)", merchantID, limit, offset))

	query := `
	SELECT id, merchant_id, amount, currency, status, card_number, cardholder_name,
		   expiry_month, expiry_year, cvv, transaction_id,
		   created_at, updated_at, processed_at
	FROM payments 
	WHERE merchant_id = ? 
	ORDER BY created_at DESC 
	LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, merchantID, limit, offset)
	if err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Failed to list payments: %s", err.Error()))
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}
	defer rows.Close()

	var payments []*models.Payment
	for rows.Next() {
		payment := &models.Payment{}
		var processedAt sql.NullTime

		err := rows.Scan(
			&payment.ID, &payment.MerchantID, &payment.Amount, &payment.Currency,
			&payment.Status, &payment.CardNumber, &payment.CardHolderName,
			&payment.ExpiryMonth, &payment.ExpiryYear, &payment.CVV,
			&payment.TransactionID, &payment.CreatedAt, &payment.UpdatedAt, &processedAt,
		)

		if err != nil {
			s.log.Error("DATABASE", fmt.Sprintf("Failed to scan payment row: %s", err.Error()))
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}

		if processedAt.Valid {
			payment.ProcessedAt = &processedAt.Time
		}

		payments = append(payments, payment)
	}

	if err = rows.Err(); err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Row iteration error: %s", err.Error()))
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Listed %d payments for merchant %s", len(payments), merchantID))
	return payments, nil
}

func (s *MySQLStore) Close() error {
	s.log.LogDatabase("CLOSE", "mysql", "Closing MySQL connection")
	return s.db.Close()
}

func (s *MySQLStore) HealthCheck() error {
	return s.db.Ping()
}
