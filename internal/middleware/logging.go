package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
)

// Key type is unexported to prevent collisions with context keys in other packages.
type key int

// requestIDKey is the key for request ID values in context.
const requestIDKey key = 0

// Logger is a custom logger that wraps the standard log.Logger.
type Logger struct {
	logger   *log.Logger
	logLevel string
}

// NewLogger initializes a new Logger instance with the given log level.
func NewLogger(logLevel string) *Logger {
	return &Logger{
		logger:   log.New(os.Stdout, "", log.LstdFlags),
		logLevel: logLevel,
	}
}

// Debug logs debug messages if the log level is set to DEBUG.
func (l *Logger) Debug(requestID, format string, v ...interface{}) {
	if l.logLevel == "DEBUG" {
		msg := fmt.Sprintf(format, v...)
		l.logger.Printf("[DEBUG] %s %s", requestID, msg)
	}
}

// Info logs information messages.
func (l *Logger) Info(requestID, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	l.logger.Printf("[INFO] %s %s", requestID, msg)
}

// RequestID is a middleware that adds a unique request ID to each request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestIDFromContext retrieves the request ID from the context.
func GetRequestIDFromContext(ctx context.Context) string {
	requestID, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return "unknown"
	}
	return requestID
}

// Example of how you might initialize the logger
var logger *Logger

// InitLogger initializes the global logger instance.
func InitLogger(logLevel string) {
	logger = NewLogger(logLevel)
}

// GetLogger retrieves the global logger instance.
func GetLogger() *Logger {
	return logger
}
