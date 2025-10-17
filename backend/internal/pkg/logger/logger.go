package logger

import (
	"log/slog"
	"os"
)

var defaultLogger *slog.Logger

// Initialize creates and configures the default logger
func Initialize(env string) *slog.Logger {
	var handler slog.Handler

	if env == "production" {
		// JSON logging for production
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
			AddSource: false,
		})
	} else {
		// Pretty text logging for development
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
			AddSource: true,
		})
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)

	return defaultLogger
}

// Get returns the default logger instance
func Get() *slog.Logger {
	if defaultLogger == nil {
		return Initialize("development")
	}
	return defaultLogger
}

// WithFields returns a new logger with additional fields
func WithFields(fields map[string]interface{}) *slog.Logger {
	logger := Get()

	for key, value := range fields {
		logger = logger.With(slog.Any(key, value))
	}

	return logger
}

// NewServiceLogger creates a logger for a specific service
func NewServiceLogger(serviceName string) *slog.Logger {
	return Get().With(slog.String("service", serviceName))
}