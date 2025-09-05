package storage

import (
	"database/sql"
	"fmt"
	"payment-gateway/internal/config"
	"payment-gateway/internal/logger"
	"payment-gateway/internal/models"

	_ "github.com/go-sql-driver/mysql"
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
        payment_id VARCHAR(36) PRIMARY KEY,
        order_id VARCHAR(36) NOT NULL,
        status VARCHAR(50) NOT NULL,
        price DECIMAL(10,2) NOT NULL,
        date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        INDEX idx_order_id (order_id),
        INDEX idx_status (status),
        INDEX idx_date (date)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
    `

	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create payments table: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", "Payments table ready")
	return nil
}

// Update SavePayment to match new fields
func (s *MySQLStore) SavePayment(payment *models.Payment) error {
	s.log.LogDatabase("INSERT", "mysql", fmt.Sprintf("Saving payment %s", payment.PaymentID))

	query := `
    INSERT INTO payments (
        payment_id, order_id, status, price, created_date, url
    ) VALUES (?, ?, ?, ?, ?, ?)
    `

	_, err := s.db.Exec(query,
		payment.PaymentID, payment.OrderID, payment.Status, payment.Price, payment.CreatedDate, payment.URL,
	)

	if err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Failed to save payment %s: %s", payment.PaymentID, err.Error()))
		return fmt.Errorf("failed to save payment: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Payment %s saved successfully", payment.PaymentID))
	return nil
}

// Update GetPayment to match new fields
func (s *MySQLStore) GetPayment(id string) (*models.Payment, error) {
	s.log.LogDatabase("SELECT", "mysql", fmt.Sprintf("Fetching payment %s", id))

	query := `
    SELECT payment_id, order_id, status, price, created_date, url
    FROM payments WHERE payment_id = ?
    `

	payment := &models.Payment{}
	err := s.db.QueryRow(query, id).Scan(
		&payment.PaymentID, &payment.OrderID, &payment.Status, &payment.Price, &payment.CreatedDate, &payment.URL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			s.log.LogDatabase("NOT_FOUND", "mysql", fmt.Sprintf("Payment %s not found", id))
			return nil, fmt.Errorf("payment not found")
		}
		s.log.Error("DATABASE", fmt.Sprintf("Failed to get payment %s: %s", id, err.Error()))
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Payment %s fetched successfully", id))
	return payment, nil
}

// Update UpdatePayment to match new fields
func (s *MySQLStore) UpdatePayment(payment *models.Payment) error {
	s.log.LogDatabase("UPDATE", "mysql", fmt.Sprintf("Updating payment %s", payment.PaymentID))

	query := `
    UPDATE payments SET
        order_id = ?, status = ?, price = ?, url = ?
    WHERE payment_id = ?
    `

	_, err := s.db.Exec(query,
		payment.OrderID, payment.Status, payment.Price, payment.URL, payment.PaymentID,
	)

	if err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Failed to update payment %s: %s", payment.PaymentID, err.Error()))
		return fmt.Errorf("failed to update payment: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Payment %s updated successfully", payment.PaymentID))
	return nil
}

// Update ListPayments to match new fields
func (s *MySQLStore) ListPayments(merchantID string, limit, offset int) ([]*models.Payment, error) {
	s.log.LogDatabase("SELECT", "mysql", fmt.Sprintf("Listing payments for order %s (limit: %d, offset: %d)", merchantID, limit, offset))

	query := `
    SELECT payment_id, order_id, status, price, created_date, url
    FROM payments 
    WHERE order_id = ? 
    ORDER BY created_date DESC 
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
		err := rows.Scan(
			&payment.PaymentID, &payment.OrderID, &payment.Status, &payment.Price, &payment.CreatedDate, &payment.URL,
		)

		if err != nil {
			s.log.Error("DATABASE", fmt.Sprintf("Failed to scan payment row: %s", err.Error()))
			return nil, fmt.Errorf("failed to scan payment: %w", err)
		}

		payments = append(payments, payment)
	}

	if err = rows.Err(); err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Row iteration error: %s", err.Error()))
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Listed %d payments for order %s", len(payments), merchantID))
	return payments, nil
}

func (s *MySQLStore) Close() error {
	s.log.LogDatabase("CLOSE", "mysql", "Closing MySQL connection")
	return s.db.Close()
}

func (s *MySQLStore) HealthCheck() error {
	return s.db.Ping()
}

func (s *MySQLStore) GetTicketByOrderID(OrderID string) (*models.Payment, error) {
	s.log.LogDatabase("SELECT", "mysql", fmt.Sprintf("Fetching payment for OrderID %s", OrderID))

	query := `
    SELECT payment_id, order_id, status, price, created_date, url
    FROM payments WHERE order_id = ?
    `

	payment := &models.Payment{}
	err := s.db.QueryRow(query, OrderID).Scan(
		&payment.PaymentID, &payment.OrderID, &payment.Status, &payment.Price, &payment.CreatedDate, &payment.URL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			s.log.LogDatabase("NOT_FOUND", "mysql", fmt.Sprintf("Payment not found for OrderID %s", OrderID))
			return nil, fmt.Errorf("payment not found")
		}
		s.log.Error("DATABASE", fmt.Sprintf("Failed to get payment %s: %s", OrderID, err.Error()))
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Payment %s fetched successfully for OrderID %s", payment.PaymentID, OrderID))
	return payment, nil
}

// SaveOrder saves an order to the database
func (s *MySQLStore) SaveOrder(order *models.Order) error {
	s.log.LogDatabase("INSERT", "mysql", fmt.Sprintf("Saving order %s", order.OrderID))

	query := `
    INSERT INTO orders (order_id, user_id, session_id, seat_ids, status, price, created_at)
    VALUES (?, ?, ?, ?, ?, ?, ?)
    `

	// Convert seat_ids slice to a string representation for storage
	// This is simplified - in a real implementation you might want to use proper JSON serialization
	seatIDsStr := fmt.Sprintf("%v", order.SeatIDs)

	_, err := s.db.Exec(query,
		order.OrderID,
		order.UserID,
		order.SessionID,
		seatIDsStr,
		order.Status,
		order.Price,
		order.CreatedAt,
	)

	if err != nil {
		s.log.Error("DATABASE", fmt.Sprintf("Failed to save order %s: %s", order.OrderID, err.Error()))
		return fmt.Errorf("failed to save order: %w", err)
	}

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Order %s saved successfully", order.OrderID))
	return nil
}

// GetOrder retrieves an order from the database by ID
func (s *MySQLStore) GetOrder(orderID string) (*models.Order, error) {
	s.log.LogDatabase("SELECT", "mysql", fmt.Sprintf("Fetching order %s", orderID))

	query := `
    SELECT order_id, user_id, session_id, seat_ids, status, price, created_at
    FROM orders WHERE order_id = ?
    `

	order := &models.Order{}
	var seatIDsStr string

	err := s.db.QueryRow(query, orderID).Scan(
		&order.OrderID,
		&order.UserID,
		&order.SessionID,
		&seatIDsStr,
		&order.Status,
		&order.Price,
		&order.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			s.log.LogDatabase("NOT_FOUND", "mysql", fmt.Sprintf("Order not found: %s", orderID))
			return nil, fmt.Errorf("order not found")
		}
		s.log.Error("DATABASE", fmt.Sprintf("Failed to get order %s: %s", orderID, err.Error()))
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// This is a simplified parsing of seat_ids from string
	// In a real implementation, you'd want proper JSON deserialization
	fmt.Sscanf(seatIDsStr, "%v", &order.SeatIDs)

	s.log.LogDatabase("SUCCESS", "mysql", fmt.Sprintf("Order %s fetched successfully", orderID))
	return order, nil
}
