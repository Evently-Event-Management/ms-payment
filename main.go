package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/stripe/stripe-go/v82"

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

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Warn("ENV", "Error loading .env file, using environment variables")
	}

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
	kafkaConsumer, err := kafka.NewOrderConsumer(cfg.Kafka.Brokers, cfg.Kafka.GroupID, store)
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

	// Initialize Stripe with API key
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		log.Warn("STRIPE", "STRIPE_SECRET_KEY environment variable not set")
		log.Warn("STRIPE", "Please set a valid Stripe API key in your .env file")
		// Don't set a default key here, let the service fail if no key is provided
	} else {
		stripe.Key = stripeKey
		log.LogProcess("STRIPE", "Stripe API initialized with key")
	}

	// Initialize services
	paymentService := services.NewPaymentService(store, kafkaProducer, log, rediswrap.NewRedis(redisClient))
	log.LogProcess("SERVICE", "Payment service initialized")

	// Initialize Stripe service
	stripeService, err := services.NewStripeService(log)
	if err != nil {
		log.Fatal("STRIPE", "Failed to initialize Stripe service: "+err.Error())
	}
	log.LogProcess("SERVICE", "Stripe service initialized")

	// Initialize handlers
	paymentHandler := handlers.NewPaymentHandler(paymentService)
	stripeHandler := handlers.NewStripeHandler(stripeService, paymentService, kafkaProducer)
	log.LogProcess("HANDLER", "All handlers initialized")

	// Start Kafka consumer in background
	go func() {
		log.LogKafka("START", "consumer", "Starting Kafka consumer goroutine")
		if err := kafkaConsumer.ConsumeOrders(context.Background(), paymentService.ProcessOrderEvent); err != nil {
			log.Error("KAFKA", "Consumer error: "+err.Error())
		}
	}()

	// Setup router
	router := setupRouter(paymentHandler, stripeHandler)
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

func setupRouter(paymentHandler *handlers.PaymentHandler, stripeHandler *handlers.StripeHandler) *gin.Engine {
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
		// Payment routes
		payments := v1.Group("/payments")
		{
			payments.POST("/process", paymentHandler.ProcessPayment)
			payments.GET("/:id", paymentHandler.GetPaymentStatus)
			payments.POST("/refund", paymentHandler.RefundPayment) // New route for refunding by order_id
		}

		// Stripe-specific routes
		stripe := v1.Group("/stripe")
		{
			stripe.POST("/validate-card", stripeHandler.ValidateCard)
			stripe.POST("/payment", stripeHandler.ProcessPayment)
			stripe.POST("/refund", stripeHandler.RefundPayment)
			stripe.GET("/payment/:id", stripeHandler.GetPaymentDetails)
			stripe.POST("/webhook", stripeHandler.HandleStripeWebhook)
		}
	}

	log.LogProcess("ROUTER", "All routes registered successfully")
	return router
}
