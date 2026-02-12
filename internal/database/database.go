package database

import (
	"fmt"
	"os"
	"time"

	pkgLogger "github.com/sjperalta/fintera-api/pkg/logger"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect establishes a connection to the PostgreSQL database
func Connect(databaseURL string) (*gorm.DB, error) {
	// Configure GORM logger
	logLevel := logger.Silent
	if os.Getenv("ENVIRONMENT") != "production" {
		logLevel = logger.Info
	}

	gormLogger := pkgLogger.NewGormLogger(
		logLevel,
		200*time.Millisecond,
	)

	// Open database connection
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true, // Improve performance
		PrepareStmt:            true, // Cache prepared statements
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
