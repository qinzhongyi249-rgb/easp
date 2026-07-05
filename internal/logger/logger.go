// Package logger provides structured logging for the EASP open source core.
// In the open source version, this is a thin wrapper around the standard library log package.
// The commercial version includes file rotation, sensitive data redaction, and Gin middleware.
package logger

import (
	"log"
)

// LogField represents a key-value pair for structured logging.
type LogField struct {
	Key   string
	Value any
}

// Field creates a LogField.
func Field(key string, value any) LogField {
	return LogField{Key: key, Value: value}
}

// Info logs an info-level message.
func Info(module, message string, fields ...LogField) {
	log.Printf("[INFO] [%s] %s %v", module, message, fields)
}

// Warn logs a warning-level message.
func Warn(module, message string, fields ...LogField) {
	log.Printf("[WARN] [%s] %s %v", module, message, fields)
}

// Error logs an error-level message.
func Error(module, message string, fields ...LogField) {
	log.Printf("[ERROR] [%s] %s %v", module, message, fields)
}

// Init initializes the logger. In the open source version, this is a no-op
// since we always write to stdout via the standard library.
func Init(_ string) error {
	return nil
}
