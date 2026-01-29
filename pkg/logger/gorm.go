package logger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm/logger"
)

type GormLogger struct {
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
}

func NewGormLogger(logLevel logger.LogLevel, slowThreshold time.Duration) *GormLogger {
	return &GormLogger{
		LogLevel:      logLevel,
		SlowThreshold: slowThreshold,
	}
}

func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		Log.Info(fmt.Sprintf(msg, data...))
	}
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		Log.Warn(fmt.Sprintf(msg, data...))
	}
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		Log.Error(fmt.Sprintf(msg, data...))
	}
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []any{
		slog.String("sql", sql),
		slog.Int64("rows", rows),
		slog.Duration("elapsed", elapsed),
	}

	if err != nil && l.LogLevel >= logger.Error {
		fields = append(fields, slog.String("error", err.Error()))
		Log.Error("SQL Error", fields...)
		return
	}

	if l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= logger.Warn {
		Log.Warn("Slow SQL", fields...)
		return
	}

	if l.LogLevel >= logger.Info {
		Log.Info("SQL", fields...)
	}
}
