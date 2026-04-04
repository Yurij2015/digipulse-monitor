package logger

import (
	"log"
	"os"
)

// Logger provides structured logging
type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
}

// New creates a new logger instance
func New() *Logger {
	return &Logger{
		infoLogger:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		errorLogger: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		debugLogger: log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.infoLogger.Println(msg)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.infoLogger.Printf(format, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.errorLogger.Println(msg)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.errorLogger.Printf(format, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.debugLogger.Println(msg)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.debugLogger.Printf(format, args...)
}