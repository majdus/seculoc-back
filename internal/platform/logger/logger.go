package logger

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	once   sync.Once
	logger *zap.Logger
	atom   zap.AtomicLevel
)

// Init initializes the logger singleton.
// It should be called once at application startup.
func Init(env string) {
	once.Do(func() {
		atom = zap.NewAtomicLevel()

		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "ts"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.CallerKey = "caller"
		encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

		var config zap.Config
		if env == "production" {
			config = zap.NewProductionConfig()
			atom.SetLevel(zap.InfoLevel)
		} else {
			config = zap.NewDevelopmentConfig()
			atom.SetLevel(zap.DebugLevel)
			encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		}

		config.EncoderConfig = encoderConfig
		config.Level = atom

		// Initial fields, if any
		// config.InitialFields = map[string]interface{}{"service": "seculoc"}

		var err error
		logger, err = config.Build(zap.AddCallerSkip(1)) // Skip 1 level to avoid always showing logger wrapper as caller
		if err != nil {
			panic(err)
		}
	})
}

// Get returns the logger singleton.
func Get() *zap.Logger {
	if logger == nil {
		// Fallback for tests or if Init wasn't called (though Init should be called)
		// Or creating a no-op logger could be an option, but for now let's just create a basic one.
		Init("development")
	}
	return logger
}

// SetLevel changes the log level dynamically.
func SetLevel(l zapcore.Level) {
	atom.SetLevel(l)
}

// WithRequestID adds a request ID to the logger context.
func WithRequestID(reqID string) *zap.Logger {
	return Get().With(zap.String("request_id", reqID))
}

// Sync flushes the logger.
func Sync() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// FromContext returns the logger from the context or the global logger if not found.
func FromContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Get()
	}
	// We assume the key is "logger" as set in middleware.
	if l, ok := ctx.Value("logger").(*zap.Logger); ok {
		return l
	}
	return Get()
}
