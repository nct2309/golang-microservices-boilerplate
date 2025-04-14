package database

import (
	"fmt"
	"golang-microservices-boilerplate/pkg/utils"
	"log"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DBConfig contains all the database configuration options
type DBConfig struct {
	URI          string
	Host         string
	Port         int
	Username     string
	Password     string
	Database     string
	SSLMode      string
	MaxIdleConns int
	MaxOpenConns int
	MaxLifetime  time.Duration
	LogLevel     logger.LogLevel
}

// DefaultDBConfig returns a default database configuration using environment variables
func DefaultDBConfig() DBConfig {
	port, _ := strconv.Atoi(utils.GetEnv("DB_PORT", "5432"))
	maxIdleConns, _ := strconv.Atoi(utils.GetEnv("DB_MAX_IDLE_CONNS", "10"))
	maxOpenConns, _ := strconv.Atoi(utils.GetEnv("DB_MAX_OPEN_CONNS", "100"))
	maxLifetime, _ := strconv.Atoi(utils.GetEnv("DB_MAX_LIFETIME", "60"))

	logLevelStr := utils.GetEnv("DB_LOG_LEVEL", "info")
	var logLevel logger.LogLevel
	switch logLevelStr {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	default:
		logLevel = logger.Info
	}

	return DBConfig{
		URI:          utils.GetEnv("DB_URI", ""),
		Host:         utils.GetEnv("DB_HOST", "localhost"),
		Port:         port,
		Username:     utils.GetEnv("DB_USER", "postgres"),
		Password:     utils.GetEnv("DB_PASSWORD", "postgres"),
		Database:     utils.GetEnv("DB_NAME", "microservices"),
		SSLMode:      utils.GetEnv("DB_SSL_MODE", "disable"),
		MaxIdleConns: maxIdleConns,
		MaxOpenConns: maxOpenConns,
		MaxLifetime:  time.Duration(maxLifetime) * time.Minute,
		LogLevel:     logLevel,
	}
}

// DatabaseConnection represents a database connection manager
type DatabaseConnection struct {
	DB     *gorm.DB
	Config DBConfig
}

// NewDatabaseConnection creates a new database connection using the provided configuration
func NewDatabaseConnection(config DBConfig) (*DatabaseConnection, error) {
	dsn := config.URI

	// Configure GORM logger
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  config.LogLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	// Open connection to the database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(config.MaxLifetime)

	return &DatabaseConnection{
		DB:     db,
		Config: config,
	}, nil
}

// Connect establishes a database connection with default configuration
func Connect() (*DatabaseConnection, error) {
	config := DefaultDBConfig()
	return NewDatabaseConnection(config)
}

// Close closes the database connection
func (dc *DatabaseConnection) Close() error {
	sqlDB, err := dc.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	return sqlDB.Close()
}

// MigrateModels runs database migrations for the provided models
func (dc *DatabaseConnection) MigrateModels(models ...interface{}) error {
	return dc.DB.AutoMigrate(models...)
}

// Ping checks if the database connection is still alive
func (dc *DatabaseConnection) Ping() error {
	sqlDB, err := dc.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	return sqlDB.Ping()
}

// Transaction executes a function within a database transaction
func (dc *DatabaseConnection) Transaction(fn func(tx *gorm.DB) error) error {
	return dc.DB.Transaction(fn)
}
