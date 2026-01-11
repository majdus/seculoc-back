package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"seculoc-back/internal/platform/logger"
)

const (
	RequestIDKey = "request_id"
	LoggerKey    = "logger"
)

// RequestLogger is a middleware that logs the request and injects the logger into the context.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Generate Request ID
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Set(RequestIDKey, rid)
		c.Header("X-Request-ID", rid)

		// 2. Create Context-Aware Logger
		// We use the singleton logger and enrich it with request_id
		ctxLogger := logger.Get().With(zap.String("request_id", rid))
		c.Set(LoggerKey, ctxLogger)

		// Inject into standard context as well for service layer
		ctx := context.WithValue(c.Request.Context(), LoggerKey, ctxLogger)
		c.Request = c.Request.WithContext(ctx)

		// 3. Process Request
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		// 4. Log Result
		end := time.Now()
		latency := end.Sub(start)

		if raw != "" {
			path = path + "?" + raw
		}

		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		fields := []zap.Field{
			zap.Int("status", statusCode),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("ip", clientIP),
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
		}

		if errorMessage != "" {
			fields = append(fields, zap.String("error", errorMessage))
		}

		if statusCode >= 500 {
			ctxLogger.Error("Server Error", fields...)
		} else if statusCode >= 400 {
			ctxLogger.Warn("Client Error", fields...)
		} else {
			ctxLogger.Info("Request", fields...)
		}
	}
}

// GetLogger extracts the logger from the context.
func GetLogger(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return logger.Get()
	}
	if l, ok := ctx.Value(LoggerKey).(*zap.Logger); ok {
		return l
	}
	return logger.Get()
}
