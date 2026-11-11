package logger

import (
	"encoding/json"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// Level represents the minimum severity threshold for emitted log entries.
type Level int8

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
	LevelFatal
	LevelOff
)

func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

// sensitiveKeys lists property substrings that must always be redacted before logging.
var sensitiveKeys = []string{"password", "secret", "token", "api_key", "apikey", "authorization"}

// Logger writes thread-safe structured JSON log lines.
type Logger struct {
	out      io.Writer
	minLevel Level
	mu       sync.Mutex
}

// New creates a structured JSON Logger writing to out.
func New(out io.Writer, minLevel Level) *Logger {
	if out == nil {
		out = io.Discard
	}
	return &Logger{
		out:      out,
		minLevel: minLevel,
	}
}

// Info logs an informational message with optional structured key-value fields.
func (l *Logger) Info(message string, fields map[string]string) {
	_, _ = l.write(LevelInfo, message, fields, false)
}

// Warn logs a warning message with optional structured key-value fields.
func (l *Logger) Warn(message string, fields map[string]string) {
	_, _ = l.write(LevelWarn, message, fields, false)
}

// Error logs an error with optional structured key-value fields.
func (l *Logger) Error(err error, fields map[string]string) {
	if err == nil {
		return
	}
	_, _ = l.write(LevelError, err.Error(), fields, false)
}

// ErrorWithStack logs an unexpected error or panic along with a goroutine stack trace.
func (l *Logger) ErrorWithStack(err error, fields map[string]string) {
	if err == nil {
		return
	}
	_, _ = l.write(LevelError, err.Error(), fields, true)
}

// Fatal logs a fatal error with stack trace and terminates the process with exit code 1.
func (l *Logger) Fatal(err error, fields map[string]string) {
	msg := "fatal error"
	if err != nil {
		msg = err.Error()
	}
	_, _ = l.write(LevelFatal, msg, fields, true)
	os.Exit(1)
}

func (l *Logger) write(level Level, message string, fields map[string]string, includeStack bool) (int, error) {
	if l == nil || level < l.minLevel {
		return 0, nil
	}

	entry := struct {
		Level      string            `json:"level"`
		Time       string            `json:"time"`
		Message    string            `json:"message"`
		Properties map[string]string `json:"properties,omitempty"`
		Trace      string            `json:"trace,omitempty"`
	}{
		Level:      level.String(),
		Time:       time.Now().UTC().Format(time.RFC3339),
		Message:    message,
		Properties: redactFields(fields),
	}

	if includeStack {
		entry.Trace = string(debug.Stack())
	}

	line, err := json.Marshal(entry)
	if err != nil {
		line = []byte(`{"level":"ERROR","message":"unable to marshal log entry"}`)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.out.Write(append(line, '\n'))
}

// Write implements io.Writer so Logger can back net/http Server.ErrorLog.
func (l *Logger) Write(message []byte) (int, error) {
	return l.write(LevelError, strings.TrimSpace(string(message)), nil, false)
}

func redactFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	sanitized := make(map[string]string, len(fields))
	for k, v := range fields {
		lower := strings.ToLower(k)
		redacted := false
		for _, secret := range sensitiveKeys {
			if strings.Contains(lower, secret) {
				redacted = true
				break
			}
		}
		if redacted {
			sanitized[k] = "[REDACTED]"
		} else {
			sanitized[k] = v
		}
	}
	return sanitized
}
