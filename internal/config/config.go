package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Redis    RedisConfig // Assuming RedisConfig is defined in the redis package
	Kafka    KafkaConfig
	Database DatabaseConfig // Added database configuration
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type RedisConfig struct {
	Addr string
}
type KafkaConfig struct {
	Brokers  []string
	GroupID  string
	Topics   TopicConfig
	MockMode bool
	Enabled  bool
}

type DatabaseConfig struct {
	Host         string
	Port         string
	Username     string
	Password     string
	Database     string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

type TopicConfig struct {
	PaymentEvents   string
	PaymentSuccess  string
	PaymentFailed   string
	PaymentRefunded string
}

func Load() *Config {
	kafkaEnabled := getEnvBool("KAFKA_ENABLED", true)
	mockMode := getEnvBool("KAFKA_MOCK_MODE", false)

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", ":8080"),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},

		Redis: RedisConfig{
			Addr: getEnv("REDIS_ADDR", "localhost:6379"),
		},

		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnv("DB_PORT", "3306"),
			Username:     getEnv("DB_USERNAME", "payment_user"),
			Password:     getEnv("DB_PASSWORD", "payment_pass"),
			Database:     getEnv("DB_NAME", "payment_gateway"),
			MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 25),
			MaxLifetime:  time.Duration(getEnvInt("DB_MAX_LIFETIME_MINUTES", 5)) * time.Minute,
		},
		Kafka: KafkaConfig{
			Brokers:  []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			GroupID:  getEnv("KAFKA_GROUP_ID", "payment-gateway-group"),
			Enabled:  kafkaEnabled,
			MockMode: mockMode,
			Topics: TopicConfig{
				PaymentEvents:   getEnv("KAFKA_TOPIC_EVENTS", "payment-events"),
				PaymentSuccess:  getEnv("KAFKA_TOPIC_SUCCESS", "payment-success"),
				PaymentFailed:   getEnv("KAFKA_TOPIC_FAILED", "payment-failed"),
				PaymentRefunded: getEnv("KAFKA_TOPIC_REFUNDED", "payment-refunded"),
			},
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getRedisAddr() string {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default Redis address
	}
	return redisAddr
}
