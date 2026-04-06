package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// Logger provides structured logging like Laravel: writes both to file AND console
type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	debugLogger *log.Logger
	logFile     *os.File
}

// New creates a new logger instance with file logging (storage/logs/monitor.log)
func New() *Logger {
	// Create logs directory like Laravel storage/logs
	logDir := "storage/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Failed to create logs directory: %v", err)
	}

	logPath := filepath.Join(logDir, "monitor.log")

	// Open log file for appending, create if not exists
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open log file: %v, using only stdout", err)
		logFile = nil
	}

	// Multi writer: write BOTH to stdout AND file
	multiWriter := io.MultiWriter(os.Stdout)
	if logFile != nil {
		multiWriter = io.MultiWriter(os.Stdout, logFile)
	}

	errorMultiWriter := io.MultiWriter(os.Stderr)
	if logFile != nil {
		errorMultiWriter = io.MultiWriter(os.Stderr, logFile)
	}

	flags := log.Ldate | log.Ltime | log.Lshortfile

	return &Logger{
		infoLogger:  log.New(multiWriter, "INFO: ", flags),
		errorLogger: log.New(errorMultiWriter, "ERROR: ", flags),
		debugLogger: log.New(multiWriter, "DEBUG: ", flags),
		logFile:     logFile,
	}
}

// Close closes the log file
func (l *Logger) Close() {
	if l.logFile != nil {
		_ = l.logFile.Close()
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
