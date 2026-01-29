package logger

import (
	"log/slog"
	"os"
)

// Logger is the global logger instance
var Log *slog.Logger

// Setup initializes the global logger based on the environment
func Setup(env string) {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	Log = slog.New(handler)
	slog.SetDefault(Log)
}

// Info logs an info message
func Info(msg string, args ...any) {
	Log.Info(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	Log.Error(msg, args...)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	Log.Debug(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	Log.Warn(msg, args...)
}
