package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"payment-gateway/internal/logger"
)

func EnhancedLogger(log *logger.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Log to our enhanced logger
		duration := param.Latency.String()
		status := fmt.Sprintf("%d", param.StatusCode)
		
		// Determine log level based on status code
		if param.StatusCode >= 500 {
			log.Error("API", fmt.Sprintf("%s %s - %s (%s) - ERROR: %s", 
				param.Method, param.Path, status, duration, param.ErrorMessage))
		} else if param.StatusCode >= 400 {
			log.Warn("API", fmt.Sprintf("%s %s - %s (%s) - Client Error", 
				param.Method, param.Path, status, duration))
		} else {
			log.LogAPI(param.Method, param.Path, status, duration)
		}

		// Also log request details for debugging
		log.Debug("REQUEST", fmt.Sprintf("IP: %s, UserAgent: %s", 
			param.ClientIP, param.Request.UserAgent()))

		// Return empty string since we're handling logging ourselves
		return ""
	})
}

func Recovery(log *logger.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			log.Error("PANIC", fmt.Sprintf("Recovered from panic: %s", err))
			c.String(http.StatusInternalServerError, fmt.Sprintf("error: %s", err))
		} else {
			log.Error("PANIC", fmt.Sprintf("Recovered from panic: %v", recovered))
			c.String(http.StatusInternalServerError, "Internal server error")
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func RateLimit(log *logger.Logger) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Every(time.Second), 100) // 100 requests per second

	return func(c *gin.Context) {
		if !limiter.Allow() {
			log.LogSecurity("RATE_LIMIT", fmt.Sprintf("Rate limit exceeded for IP: %s", c.ClientIP()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": "1s",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func SecurityHeaders(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		
		// Log security events
		if c.GetHeader("X-Forwarded-For") != "" {
			log.LogSecurity("PROXY_REQUEST", fmt.Sprintf("Request via proxy from: %s", c.GetHeader("X-Forwarded-For")))
		}
		
		c.Next()
	}
}
