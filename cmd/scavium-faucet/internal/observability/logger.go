// Package observability provides lightweight structured logging helpers.
package observability

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"
)

// Logger writes structured JSON log entries.
type Logger struct {
	logger *log.Logger
}

// NewLogger creates a JSON logger that writes one entry per line to w.
func NewLogger(w io.Writer) *Logger {
	return &Logger{
		logger: log.New(w, "", 0),
	}
}

// DefaultLogger creates a JSON logger that writes to stdout.
func DefaultLogger() *Logger {
	return NewLogger(os.Stdout)
}

// Info writes an informational log entry.
func (l *Logger) Info(message string, fields map[string]any) {
	l.write("info", message, fields)
}

// Error writes an error log entry.
func (l *Logger) Error(message string, fields map[string]any) {
	l.write("error", message, fields)
}

func (l *Logger) write(level, message string, fields map[string]any) {
	entry := map[string]any{
		"time":    time.Now().UTC().Format(time.RFC3339),
		"level":   level,
		"message": message,
	}
	for key, value := range fields {
		entry[key] = value
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		l.logger.Printf(`{"level":"error","message":"log encode failed"}`)
		return
	}
	l.logger.Print(string(encoded))
}
