package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"

	"payment-gateway/internal/config"
	"payment-gateway/internal/handlers"
	"payment-gateway/internal/kafka"
	"payment-gateway/internal/logger"
	"payment-gateway/internal/middleware"
	rediswrap "payment-gateway/internal/redis"
	"payment-gateway/internal/services"
	"payment-gateway/internal/storage"

	"github.com/gin-gonic/gin"
)

// Global logger instance
var log *logger.Logger

func main() {
	log = logger.NewLogger()
	defer log.Close()
	//err := godotenv.Load()
	//if err != nil {
	//	log.Fatal("ENV", "Error loading .env file")
	//}
	log.LogProcess("STARTUP", "Payment Gateway starting up...")
	log.Info("SYSTEM", "Initializing components...")

	// Load configuration
	cfg := config.Load()
	log.Info("CONFIG", "Configuration loaded successfully")

	log.LogProcess("DATABASE", "Initializing MySQL database...")
	store, err := storage.NewMySQLStore(cfg.Database, log)
	if err != nil {
		log.Fatal("DATABASE", "Failed to initialize MySQL: "+err.Error())
	}
	defer store.Close()
	log.LogDatabase("INIT", "mysql", "MySQL storage initialized successfully")

	// Initialize Kafka
	log.LogProcess("KAFKA", "Initializing Kafka producer...")
	kafkaProducer, err := kafka.NewProducer(cfg.Kafka.Brokers, true, log)

	if err != nil {
		log.Fatal("KAFKA", "Failed to create Kafka producer: "+err.Error())
	}
	defer kafkaProducer.Close()
	log.LogKafka("INIT", "producer", "Kafka producer initialized successfully")

	log.LogProcess("KAFKA", "Initializing Kafka consumer...")
	kafkaConsumer, err := kafka.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.GroupID)
	if err != nil {
		log.Fatal("KAFKA", "Failed to create Kafka consumer: "+err.Error())
	}
	defer kafkaConsumer.Close()
	log.LogKafka("INIT", "consumer", "Kafka consumer initialized successfully")
	redisAddr := os.Getenv("REDIS_ADDR")
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	log.LogProcess("SERVICE", " Redis connection successful")
	// Initialize services
	paymentService := services.NewPaymentService(store, kafkaProducer, log, rediswrap.NewRedis(redisClient))
	log.LogProcess("SERVICE", "Payment service initialized")

	// Initialize handlers
	paymentHandler := handlers.NewPaymentHandler(paymentService)
	log.LogProcess("HANDLER", "Payment handler initialized")

	// Start Kafka consumer in background
	go func() {
		log.LogKafka("START", "consumer", "Starting Kafka consumer goroutine")
		if err := kafkaConsumer.ConsumePayments(context.Background(), paymentService.ProcessPaymentEvent); err != nil {
			log.Error("KAFKA", "Consumer error: "+err.Error())
		}
	}()

	// Setup router
	router := setupRouter(paymentHandler)
	log.LogProcess("ROUTER", "HTTP router configured")

	// Create server
	srv := &http.Server{
		Addr:         ":8085",
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.LogProcess("SERVER", "Starting HTTP server on port "+cfg.Server.Port)
		log.Info("STARTUP", "🚀 Payment Gateway is ready to accept requests!")
		log.Info("STARTUP", "📊 Health check available at: http://localhost"+cfg.Server.Port+"/health")
		log.Info("STARTUP", "💳 Payment API available at: http://localhost"+cfg.Server.Port+"/api/v1/payments")

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("SERVER", "Server failed to start: "+err.Error())
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Warn("SHUTDOWN", "Received shutdown signal, initiating graceful shutdown...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("SHUTDOWN", "Server forced to shutdown: "+err.Error())
	}

	log.Info("SHUTDOWN", "✅ Payment Gateway shutdown completed successfully")
}

func setupRouter(paymentHandler *handlers.PaymentHandler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	router.Use(middleware.EnhancedLogger(log))
	router.Use(middleware.Recovery(log))
	router.Use(middleware.CORS())
	router.Use(middleware.RateLimit(log))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		log.LogAPI("GET", "/health", "200", "0ms")
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"service":   "payment-gateway",
			"version":   "1.0.0",
		})
	})

	// API routes
	v1 := router.Group("/api/v1")
	{
		payments := v1.Group("/payments")
		{
			payments.POST("/process", paymentHandler.ProcessPayment)
			payments.GET("/:id", paymentHandler.GetPayment)
			payments.GET("/:id/status", paymentHandler.GetPaymentStatus)
			payments.POST("/:id/refund", paymentHandler.RefundPayment)
			payments.POST("/OTP", paymentHandler.OTP)
			payments.POST("/validate", paymentHandler.ValidateOTP)
		}
	}

	log.LogProcess("ROUTER", "All routes registered successfully")
	return router
}
