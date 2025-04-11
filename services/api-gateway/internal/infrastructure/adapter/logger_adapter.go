package adapter

import (
	"golang-microservices-boilerplate/pkg/core/logger"
	"log"
	"os"
)

// StdLoggerAdapter provides compatibility when structured logger initialization fails
// by adapting a standard logger to the core.Logger interface
type StdLoggerAdapter struct {
	stdLogger *log.Logger
}

// NewStdLoggerAdapter creates a new adapter for standard logger
func NewStdLoggerAdapter(prefix string) *StdLoggerAdapter {
	stdLogger := log.New(os.Stdout, prefix, log.LstdFlags|log.Lshortfile)
	return &StdLoggerAdapter{stdLogger: stdLogger}
}

// Debug logs a debug message
func (a *StdLoggerAdapter) Debug(msg string, args ...interface{}) {
	a.stdLogger.Printf("DEBUG: %s, %v", msg, args)
}

// Info logs an info message
func (a *StdLoggerAdapter) Info(msg string, args ...interface{}) {
	a.stdLogger.Printf("INFO: %s, %v", msg, args)
}

// Warn logs a warning message
func (a *StdLoggerAdapter) Warn(msg string, args ...interface{}) {
	a.stdLogger.Printf("WARN: %s, %v", msg, args)
}

// Error logs an error message
func (a *StdLoggerAdapter) Error(msg string, args ...interface{}) {
	a.stdLogger.Printf("ERROR: %s, %v", msg, args)
}

// Fatal logs a fatal message and exits
func (a *StdLoggerAdapter) Fatal(msg string, args ...interface{}) {
	a.stdLogger.Fatalf("FATAL: %s, %v", msg, args)
}

// With returns a logger with the given context values
func (a *StdLoggerAdapter) With(args ...interface{}) logger.Logger {
	return a // No-op implementation for simplicity
}

// Named returns a logger with the given name
func (a *StdLoggerAdapter) Named(name string) logger.Logger {
	newLogger := log.New(os.Stdout, "["+name+"] ", log.LstdFlags|log.Lshortfile)
	return &StdLoggerAdapter{stdLogger: newLogger}
}
